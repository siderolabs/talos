// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta

import (
	"encoding/pem"
	"fmt"
	"slices"

	"github.com/siderolabs/crypto/x509"
)

// PEMTypeCertificate is the PEM block type for an X.509 certificate.
const PEMTypeCertificate = "CERTIFICATE"

// CertificateAndKey represents a PEM-encoded certificate and key pair.
type CertificateAndKey struct {
	Cert string `yaml:"cert,omitempty"`
	Key  string `yaml:"key,omitempty"`
}

// Validate the certificate and key pair.
func (c *CertificateAndKey) Validate(tryLoading bool) error {
	if c == nil {
		return nil
	}

	if c.Cert == "" {
		return fmt.Errorf("certificate is required")
	}

	if c.Key == "" {
		return fmt.Errorf("key is required")
	}

	if certErr := AssertValidPEM([]byte(c.Cert), PEMTypeCertificate); certErr != nil {
		return fmt.Errorf("certificate: %w", certErr)
	}

	if keyErr := AssertValidPEM([]byte(c.Key)); keyErr != nil {
		return fmt.Errorf("key: %w", keyErr)
	}

	if tryLoading {
		x509pair := c.ToX509()

		if _, err := x509pair.GetCert(); err != nil {
			return fmt.Errorf("certificate is invalid: %w", err)
		}

		if _, err := x509pair.GetKey(); err != nil {
			return fmt.Errorf("key is invalid: %w", err)
		}
	}

	return nil
}

// ToX509 converts to the siderolabs/crypto x509.Certificate and private key.
func (c *CertificateAndKey) ToX509() *x509.PEMEncodedCertificateAndKey {
	if c == nil {
		return nil
	}

	return &x509.PEMEncodedCertificateAndKey{
		Crt: []byte(c.Cert),
		Key: []byte(c.Key),
	}
}

// AssertValidPEM checks if the data contains at least one valid PEM block.
//
// If expectedTypes is non-empty, every decoded block must have one of the
// expected block types (e.g. PEMTypeCertificate).
func AssertValidPEM(data []byte, expectedTypes ...string) error {
	var numBlocks int

	for {
		var pemBlock *pem.Block

		pemBlock, data = pem.Decode(data)
		if pemBlock == nil {
			break
		}

		if len(expectedTypes) > 0 && !slices.Contains(expectedTypes, pemBlock.Type) {
			return fmt.Errorf("unexpected PEM block type %q, expected one of %q", pemBlock.Type, expectedTypes)
		}

		numBlocks++
	}

	if numBlocks == 0 {
		return fmt.Errorf("no PEM blocks found")
	}

	return nil
}
