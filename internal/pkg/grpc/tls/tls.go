/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/talos-systems/talos/pkg/userdata"
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
func NewConfig(t Type, data *userdata.OSSecurity) (config *tls.Config, err error) {
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

	switch t {
	case Mutual:
		config.ClientAuth = tls.RequireAndVerifyClientCert
	case ServerOnly:
		config.ClientAuth = tls.NoClientCert
	}

	return config, nil
}
