package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// Type represents the TLS authentication type.
type Type int

const (
	// Mutual configures the server's policy for TLS Client Authentication to
	// mutual TLS.
	Mutual Type = 1 << iota
	// ServerOnly configures the server's policy for TLS Client Authentication
	// to server only.
	ServerOnly
)

// NewConfig initializes a TLS config for the specified type.
func NewConfig(t Type, data *userdata.Security) (config *tls.Config, err error) {
	certPool := x509.NewCertPool()
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}
	if ok := certPool.AppendCertsFromPEM(data.CA.Crt); !ok {
		return nil, fmt.Errorf("failed to append client certs")
	}

	certificate, err := tls.X509KeyPair(data.Identity.Crt, data.Identity.Key)
	if err != nil {
		return nil, fmt.Errorf("could not load server key pair: %s", err)
	}

	config = &tls.Config{
		// Set the root certificate authorities to use the self-signed
		// certificate.
		RootCAs: certPool,
		// Validate certificates against the provided CA.
		ClientCAs: certPool,
		// Present the certificate to the other side.
		Certificates: []tls.Certificate{certificate},
		// Perfect Forward Secrecy.
		CurvePreferences: []tls.CurveID{tls.X25519},
		CipherSuites:     []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
		// Force the above cipher suites.
		PreferServerCipherSuites: true,
		// TLS 1.2
		MinVersion: tls.VersionTLS12,
	}

	switch t {
	case Mutual:
		config.ClientAuth = tls.RequireAndVerifyClientCert
	case ServerOnly:
		config.ClientAuth = tls.NoClientCert
	}

	return config, nil
}
