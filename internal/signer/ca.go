package signer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
)

type DevCA struct {
	certificate *x509.Certificate
	privateKey  *ecdsa.PrivateKey
}

var _ Issuer = (*DevCA)(nil)

// NewDevCA creates an ephemeral development CA.
// Do not use this CA in production.
func NewDevCA() (*DevCA, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	now := time.Now().UTC()

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "SPIFFE Demo Development CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	return &DevCA{
		certificate: cert,
		privateKey:  key,
	}, nil
}

// Issue signs the public key from a kubelet-generated CSR.
// This is for local development only; it does not authorize the pod identity.
func (ca *DevCA) Issue(
	pcr *certificatesv1.PodCertificateRequest,
) (*Result, error) {
	if pcr == nil {
		return nil, fmt.Errorf("PodCertificateRequest is nil")
	}

	if pcr.Spec.MaxExpirationSeconds == nil ||
		*pcr.Spec.MaxExpirationSeconds <= 0 {
		return nil, fmt.Errorf("invalid maxExpirationSeconds")
	}

	maxExpirationSeconds := *pcr.Spec.MaxExpirationSeconds

	csr, err := x509.ParseCertificateRequest(pcr.Spec.StubPKCS10Request)
	if err != nil {
		return nil, fmt.Errorf("parse CSR: %w", err)
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("verify CSR signature: %w", err)
	}

	// The demo currently supports ECDSA P-256 only.
	pub, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, fmt.Errorf("unsupported public key: expected ECDSA P-256")
	}

	// Development-only identity mapping. Do not trust these fields
	// without validating the referenced Kubernetes objects in production.
	spiffeID, err := url.Parse(fmt.Sprintf(
		"spiffe://weristdominik.de/ns/%s/sa/%s",
		pcr.Namespace,
		pcr.Spec.ServiceAccountName,
	))
	if err != nil {
		return nil, fmt.Errorf("build SPIFFE ID: %w", err)
	}

	now := time.Now().UTC()
	notBefore := now.Add(-30 * time.Second)
	notAfter := notBefore.Add(
		time.Duration(maxExpirationSeconds) * time.Second,
	)

	serial, err := rand.Int(
		rand.Reader,
		new(big.Int).Lsh(big.NewInt(1), 128),
	)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{},
		URIs:         []*url.URL{spiffeID},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	leafDER, err := x509.CreateCertificate(
		rand.Reader,
		template,
		ca.certificate,
		csr.PublicKey,
		ca.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("sign workload certificate: %w", err)
	}

	// Kubelet generated and retains the private key. We return only the
	// leaf certificate and CA certificate chain.
	chain := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: leafDER,
	})
	chain = append(chain, pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.certificate.Raw,
	})...)

	// refresh after 80% of total lifetime is reached
	refreshAt := notBefore.Add(
		time.Duration(maxExpirationSeconds) * 4 * time.Second / 5,
	)

	return &Result{
		CertificateChain: chain,
		NotBefore:        notBefore,
		NotAfter:         notAfter,
		BeginRefreshAt:   refreshAt,
	}, nil
}

// TrustBundlePEM returns the CA certificate in PEM format.
func (ca *DevCA) TrustBundlePEM() []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.certificate.Raw,
	})
}
