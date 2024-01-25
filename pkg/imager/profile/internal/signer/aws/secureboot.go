// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
)

// SecureBootSigner implements pesign.CertificateSigner interface.
type SecureBootSigner struct {
	keySigner *KeySigner
	cert      *x509.Certificate
}

// Verify interface.
var _ pesign.CertificateSigner = (*SecureBootSigner)(nil)

// Signer returns the signer.
func (s *SecureBootSigner) Signer() crypto.Signer {
	return s.keySigner
}

// Certificate returns the certificate.
func (s *SecureBootSigner) Certificate() *x509.Certificate {
	return s.cert
}

// NewSecureBootSigner creates a new SecureBootSigner.
func NewSecureBootSigner(ctx context.Context, kmsKeyID, awsRegion, certPath string) (*SecureBootSigner, error) {
	keySigner, err := NewPCRSigner(ctx, kmsKeyID, awsRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize certificate key signer (kms): %w", err)
	}

	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	certBlock, _ := pem.Decode(certData)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode certificate")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &SecureBootSigner{
		keySigner: keySigner,
		cert:      cert,
	}, nil
}
