package signer

import (
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
)

type Result struct {
	CertificateChain []byte
	NotBefore        time.Time
	NotAfter         time.Time
	BeginRefreshAt   time.Time
}

type Issuer interface {
	Issue(*certificatesv1.PodCertificateRequest) (*Result, error)
}
