// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package file implements SecureBoot/PCR signers via plain filesystem files.
package file

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/siderolabs/talos/internal/pkg/secureboot/measure"
)

// PCRSigner implements measure.RSAKey interface.
type PCRSigner struct {
	key *rsa.PrivateKey
}

// Verify interface.
var _ measure.RSAKey = (*PCRSigner)(nil)

// PublicRSAKey returns the public key.
func (s *PCRSigner) PublicRSAKey() *rsa.PublicKey {
	return &s.key.PublicKey
}

// Public returns the public key.
func (s *PCRSigner) Public() crypto.PublicKey {
	return s.PublicRSAKey()
}

// Sign implements the crypto.Signer interface.
func (s *PCRSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	return s.key.Sign(rand, digest, opts)
}

// NewPCRSigner creates a new PCR signer from the private key file.
func NewPCRSigner(keyPath string) (*PCRSigner, error) {
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
		return nil, fmt.Errorf("failed to parse private RSA key: %v", err)
	}

	return &PCRSigner{rsaKey}, nil
}
