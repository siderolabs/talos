// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pesign implements the PE (portable executable) signing.
package pesign

import (
	"crypto"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/foxboron/go-uefi/authenticode"
)

// Signer sigs PE (portable executable) files.
type Signer struct {
	provider CertificateSigner
}

// CertificateSigner is a provider of the certificate and the signer.
type CertificateSigner interface {
	Signer() crypto.Signer
	Certificate() *x509.Certificate
}

// NewSigner creates a new Signer.
func NewSigner(provider CertificateSigner) (*Signer, error) {
	return &Signer{
		provider: provider,
	}, nil
}

// Sign signs the input file and writes the output to the output file.
func (s *Signer) Sign(input, output string) error {
	in, err := os.Open(input)
	if err != nil {
		return err
	}

	defer in.Close() //nolint:errcheck

	pecoff, err := authenticode.Parse(in)
	if err != nil {
		return fmt.Errorf("error parsing binary: %w", err)
	}

	_, err = pecoff.Sign(s.provider.Signer(), s.provider.Certificate())
	if err != nil {
		return fmt.Errorf("error signing binary: %w", err)
	}

	return os.WriteFile(output, pecoff.Bytes(), 0o600)
}
