// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package aws

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io"
	"math/big"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"

	"github.com/siderolabs/talos/internal/pkg/secureboot/measure"
)

// KeySigner implements measure.RSAKey interface.
//
// KeySigner wraps Azure APIs to provide public key and crypto.Signer interface out of Azure Key Vault RSA key.
type KeySigner struct {
	keyName string
	mode    mode

	client    *kms.Client
	publicKey *rsa.PublicKey
}

var algMap = map[mode]map[crypto.Hash]types.SigningAlgorithmSpec{
	rsaPKCS1v15: {
		crypto.SHA256: types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
		crypto.SHA384: types.SigningAlgorithmSpecRsassaPkcs1V15Sha384,
		crypto.SHA512: types.SigningAlgorithmSpecRsassaPkcs1V15Sha512,
	},
	rsaPSS: {
		crypto.SHA256: types.SigningAlgorithmSpecRsassaPssSha256,
		crypto.SHA384: types.SigningAlgorithmSpecRsassaPssSha384,
		crypto.SHA512: types.SigningAlgorithmSpecRsassaPssSha512,
	},
	ecdsa: {
		crypto.SHA256: types.SigningAlgorithmSpecEcdsaSha256,
		crypto.SHA384: types.SigningAlgorithmSpecEcdsaSha384,
		crypto.SHA512: types.SigningAlgorithmSpecEcdsaSha512,
	},
}

type mode string

const (
	rsaPKCS1v15 mode = "pkcs1v15"
	rsaPSS      mode = "pss"
	ecdsa       mode = "ecdsa"
)

// PublicRSAKey returns the public key.
func (s *KeySigner) PublicRSAKey() *rsa.PublicKey {
	return s.publicKey
}

// Public returns the public key.
func (s *KeySigner) Public() crypto.PublicKey {
	return s.PublicRSAKey()
}

// Sign implements the crypto.Signer interface.
func (s *KeySigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	mode := s.mode

	inner := algMap[mode]
	if inner == nil {
		return nil, fmt.Errorf("mode not supported")
	}

	hf := crypto.SHA256

	if opts != nil {
		hf = opts.HashFunc()
	}

	algorithm := inner[hf]
	if algorithm == "" {
		return nil, fmt.Errorf("algorithm not supported")
	}

	resp, err := s.client.Sign(context.Background(), &kms.SignInput{
		KeyId:            &s.keyName,
		Message:          digest,
		MessageType:      types.MessageTypeDigest,
		SigningAlgorithm: algorithm,
	})
	if err != nil {
		return nil, err
	}

	return resp.Signature, nil
}

// Verify interface.
var _ measure.RSAKey = (*KeySigner)(nil)

// NewPCRSigner creates a new PCR signer from AWS settings.
func NewPCRSigner(ctx context.Context, kmsKeyID, awsRegion string) (*KeySigner, error) {
	client, err := getKmsClient(ctx, awsRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to build AWS kms client: %w", err)
	}

	keyResponse, err := client.GetPublicKey(ctx, &kms.GetPublicKeyInput{
		KeyId: &kmsKeyID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	if keyResponse.KeyUsage != "SIGN_VERIFY" {
		return nil, fmt.Errorf("key usage is not SIGN_VERIFY")
	}

	switch keyResponse.KeySpec { //nolint:exhaustive
	case types.KeySpecRsa2048, types.KeySpecRsa3072, types.KeySpecRsa4096:
		// expected, continue
	default:
		return nil, fmt.Errorf("key type is not RSA")
	}

	parsedKey, err := x509.ParsePKIXPublicKey(keyResponse.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("Public key is not valid: %w", err)
	}

	rsaKey := parsedKey.(*rsa.PublicKey) //nolint:errcheck
	if rsaKey.E == 0 {
		return nil, fmt.Errorf("property e is empty")
	}

	if rsaKey.N.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("property N is empty")
	}

	return &KeySigner{
		keyName: kmsKeyID,
		mode:    rsaPKCS1v15, // TODO: make this configurable

		publicKey: rsaKey,
		client:    client,
	}, nil
}
