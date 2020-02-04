// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
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

// ConfigOptionFunc describes a configuration option function for the TLS config
type ConfigOptionFunc func(*tls.Config) error

// WithClientAuthType declares the server's policy regardling TLS Client Authentication
func WithClientAuthType(t Type) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		switch t {
		case Mutual:
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
		case ServerOnly:
			cfg.ClientAuth = tls.NoClientCert
		default:
			return fmt.Errorf("unhandled client auth type %+v", t)
		}

		return nil
	}
}

// WithServerCertificateProvider declares a dynamic provider for the server
// certificate.
//
// NOTE: specifying this option will CLEAR any configured Certificates, since
// they would otherwise override this option.
func WithServerCertificateProvider(p CertificateProvider) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		if p == nil {
			return errors.New("no provider")
		}

		cfg.Certificates = nil
		cfg.GetCertificate = p.GetCertificate

		return nil
	}
}

// WithClientCertificateProvider declares a dynamic provider for the client
// certificate.
//
// NOTE: specifying this option will CLEAR any configured Certificates, since
// they would otherwise override this option.
func WithClientCertificateProvider(p CertificateProvider) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		if p == nil {
			return errors.New("no provider")
		}

		cfg.Certificates = nil
		cfg.GetClientCertificate = p.GetClientCertificate

		return nil
	}
}

// WithKeypair declares a specific TLS keypair to be used.  This can be called
// multiple times to add additional keypairs.
func WithKeypair(cert tls.Certificate) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		cfg.Certificates = append(cfg.Certificates, cert)
		return nil
	}
}

// WithCACertPEM declares a PEM-encoded CA Certificate to be used.
func WithCACertPEM(ca []byte) func(*tls.Config) error {
	return func(cfg *tls.Config) error {
		if len(ca) == 0 {
			return errors.New("no CA cert provided")
		}

		if ok := cfg.ClientCAs.AppendCertsFromPEM(ca); !ok {
			return errors.New("failed to append CA certificate to ClientCAs pool")
		}

		if ok := cfg.RootCAs.AppendCertsFromPEM(ca); !ok {
			return errors.New("failed to append CA certificate to RootCAs pool")
		}

		return nil
	}
}

func defaultConfig() *tls.Config {
	return &tls.Config{
		RootCAs:   x509.NewCertPool(),
		ClientCAs: x509.NewCertPool(),
		// Use the X25519 elliptic curve for the ECDHE key exchange algorithm.
		CurvePreferences:       []tls.CurveID{tls.X25519},
		SessionTicketsDisabled: true,
		// TLS protocol, ECDHE key exchange algorithm, ECDSA digital signature algorithm, AES_256_GCM bulk encryption algorithm, and SHA384 hash algorithm.
		CipherSuites: []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
		// Force the above cipher suites.
		PreferServerCipherSuites: true,
		// TLS 1.2
		MinVersion: tls.VersionTLS12,
	}
}

// New returns a new TLS Configuration modified by any provided configuration options
func New(opts ...ConfigOptionFunc) (cfg *tls.Config, err error) {
	cfg = defaultConfig()

	for _, f := range opts {
		if err = f(cfg); err != nil {
			return
		}
	}

	return
}
