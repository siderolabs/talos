// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package file

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
)

// SecureBootSigner implements pesign.CertificateSigner interface.
type SecureBootSigner struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

// Verify interface.
var _ pesign.CertificateSigner = (*SecureBootSigner)(nil)

// Signer returns the signer.
func (s *SecureBootSigner) Signer() crypto.Signer {
	return s.key
}

// Certificate returns the certificate.
func (s *SecureBootSigner) Certificate() *x509.Certificate {
	return s.cert
}

// NewSecureBootSigner creates a new SecureBootSigner.
func NewSecureBootSigner(certPath, keyPath string) (*SecureBootSigner, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	// convert private key to rsa.PrivateKey
	rsaPrivateKeyBlock, _ := pem.Decode(keyData)
	if rsaPrivateKeyBlock == nil {
		return nil, errors.New("failed to decode private key")
	}

	rsaKey, err := x509.ParsePKCS1PrivateKey(rsaPrivateKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private RSA key: %w", err)
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return nil, errors.New("failed to decode certificate")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &SecureBootSigner{
		key:  rsaKey,
		cert: cert,
	}, nil
}
