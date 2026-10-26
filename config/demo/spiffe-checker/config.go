package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"time"
)

type Config struct {
	CredentialFile  string
	TrustBundleFile string
	ReloadInterval  time.Duration
	MinTLSVersion   uint16

	ListenAddress  string
	ExpectedClient string

	TargetURL       string
	ExpectedServer  string
	RequestInterval time.Duration
}

type TLSMaterial struct {
	Certificate tls.Certificate
	Roots       *x509.CertPool
	Fingerprint [32]byte
}

type Checker struct {
	Config  Config
	current atomic.Pointer[TLSMaterial]
}

func parseTLSVersion(version string) (uint16, error) {
	switch version {
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS version %q; use 1.2 or 1.3", version)
	}
}

func validSPIFFEID(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil &&
		u.Scheme == "spiffe" &&
		u.Host != "" &&
		u.User == nil &&
		u.RawQuery == "" &&
		!u.ForceQuery &&
		u.Fragment == "" &&
		u.Opaque == ""
}

func (c Config) Validate(mode string) error {
	if c.CredentialFile == "" || c.TrustBundleFile == "" {
		return errors.New("credential and trust-bundle paths are required")
	}
	if c.ReloadInterval <= 0 {
		return errors.New("reload interval must be positive")
	}

	switch mode {
	case "server":
		if c.ListenAddress == "" || !validSPIFFEID(c.ExpectedClient) {
			return errors.New("server requires --listen and a valid --expected-client")
		}
	case "client":
		u, err := url.Parse(c.TargetURL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return errors.New("client requires a valid HTTPS --target")
		}
		if !validSPIFFEID(c.ExpectedServer) {
			return errors.New("client requires a valid --expected-server SPIFFE ID")
		}
		if c.RequestInterval <= 0 {
			return errors.New("request interval must be positive")
		}
	default:
		return errors.New("mode must be server or client")
	}
	return nil
}

func NewChecker(cfg Config) (*Checker, error) {
	c := &Checker{Config: cfg}
	if err := c.Reload(); err != nil {
		return nil, fmt.Errorf("initial TLS load: %w", err)
	}
	return c, nil
}

func (c *Checker) load() (*TLSMaterial, error) {
	credential, err := os.ReadFile(c.Config.CredentialFile)
	if err != nil {
		return nil, fmt.Errorf("read credential: %w", err)
	}

	cert, err := tls.X509KeyPair(credential, credential)
	if err != nil {
		return nil, fmt.Errorf("parse workload credential: %w", err)
	}
	if len(cert.Certificate) == 0 {
		return nil, errors.New("credential contains no certificate")
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse leaf certificate: %w", err)
	}

	bundle, err := os.ReadFile(c.Config.TrustBundleFile)
	if err != nil {
		return nil, fmt.Errorf("read trust bundle: %w", err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(bundle) {
		return nil, errors.New("trust bundle contains no usable CA certificates")
	}

	hash := sha256.New()
	hash.Write(credential)
	hash.Write([]byte{0})
	hash.Write(bundle)

	var fingerprint [32]byte
	copy(fingerprint[:], hash.Sum(nil))

	return &TLSMaterial{
		Certificate: cert,
		Roots:       roots,
		Fingerprint: fingerprint,
	}, nil
}

func (c *Checker) Reload() error {
	next, err := c.load()
	if err != nil {
		return err
	}

	old := c.current.Load()
	if old != nil && old.Fingerprint == next.Fingerprint {
		return nil
	}

	c.current.Store(next)
	fmt.Println("TLS credentials/trust bundle loaded or reloaded")
	return nil
}

func (c *Checker) Watch(ctx context.Context) {
	ticker := time.NewTicker(c.Config.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.Reload(); err != nil {
				fmt.Printf("TLS reload failed; retaining last valid configuration: %v\n", err)
			}
		}
	}
}

func (c *Checker) VerifyPeer(
	state tls.ConnectionState,
	expectedID string,
	usage x509.ExtKeyUsage,
) error {
	if len(state.PeerCertificates) == 0 {
		return errors.New("peer did not provide a certificate")
	}

	active := c.current.Load()
	if active == nil {
		return errors.New("TLS material not loaded")
	}

	leaf := state.PeerCertificates[0]
	intermediates := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         active.Roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{usage},
	}); err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	for _, uri := range leaf.URIs {
		if uri.String() == expectedID {
			return nil
		}
	}
	return fmt.Errorf("unexpected SPIFFE identity; expected %s", expectedID)
}

func (c *Checker) serverTLS() *tls.Config {
	return &tls.Config{
		MinVersion:             c.Config.MinTLSVersion,
		ClientAuth:             tls.RequireAnyClientCert,
		SessionTicketsDisabled: true,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			active := c.current.Load()
			if active == nil {
				return nil, errors.New("TLS material not loaded")
			}
			return &active.Certificate, nil
		},
		VerifyConnection: func(state tls.ConnectionState) error {
			return c.VerifyPeer(state, c.Config.ExpectedClient, x509.ExtKeyUsageClientAuth)
		},
	}
}

func (c *Checker) clientTLS() *tls.Config {
	return &tls.Config{
		MinVersion:             c.Config.MinTLSVersion,
		InsecureSkipVerify:     true,
		SessionTicketsDisabled: true,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			active := c.current.Load()
			if active == nil {
				return nil, errors.New("TLS material not loaded")
			}
			return &active.Certificate, nil
		},
		VerifyConnection: func(state tls.ConnectionState) error {
			return c.VerifyPeer(state, c.Config.ExpectedServer, x509.ExtKeyUsageServerAuth)
		},
	}
}
