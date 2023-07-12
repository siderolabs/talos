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

	"github.com/foxboron/go-uefi/efi"
	siderox509 "github.com/siderolabs/crypto/x509"
)

// Signer sigs PE (portable executable) files.
type Signer struct {
	key  crypto.Signer
	cert *x509.Certificate
}

// NewSigner creates a new Signer.
func NewSigner(certFile, keyFile string) (*Signer, error) {
	pem, err := siderox509.NewCertificateAndKeyFromFiles(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	cert, err := pem.GetCert()
	if err != nil {
		return nil, err
	}

	key, err := pem.GetKey()
	if err != nil {
		return nil, err
	}

	if signer, ok := key.(crypto.Signer); ok {
		return &Signer{
			key:  signer,
			cert: cert,
		}, nil
	}

	return nil, fmt.Errorf("key is not a crypto.Signer")
}

// Sign signs the input file and writes the output to the output file.
func (s *Signer) Sign(input, output string) error {
	unsigned, err := os.ReadFile(input)
	if err != nil {
		return err
	}

	signed, err := efi.SignEFIExecutable(s.key, s.cert, unsigned)
	if err != nil {
		return err
	}

	return os.WriteFile(output, signed, 0o600)
}
