// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"context"
	"crypto"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"

	"github.com/siderolabs/talos/internal/pkg/measure"
)

// KeySigner implements measure.RSAKey interface.
//
// KeySigner wraps Azure APIs to provide public key and crypto.Signer interface out of Azure Key Vault RSA key.
type KeySigner struct {
	keyName, keyVersion string

	client    *azkeys.Client
	publicKey *rsa.PublicKey
}

// PublicRSAKey returns the public key.
func (s *KeySigner) PublicRSAKey() *rsa.PublicKey {
	return s.publicKey
}

// Public returns the public key.
func (s *KeySigner) Public() crypto.PublicKey {
	return s.PublicRSAKey()
}

// Sign implements the crypto.Signer interface.
func (s *KeySigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	params := azkeys.SignParameters{
		Value: digest,
	}

	hf := crypto.SHA256

	if opts != nil {
		hf = opts.HashFunc()
	}

	switch hf { //nolint:exhaustive
	case crypto.SHA256:
		params.Algorithm = new(azkeys.SignatureAlgorithmRS256)
	case crypto.SHA384:
		params.Algorithm = new(azkeys.SignatureAlgorithmRS384)
	case crypto.SHA512:
		params.Algorithm = new(azkeys.SignatureAlgorithmRS512)
	default:
		return nil, errors.New("unsupported hashing function")
	}

	resp, err := s.client.Sign(context.Background(), s.keyName, s.keyVersion, params, &azkeys.SignOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	return resp.Result, nil
}

// Verify interface.
var _ measure.RSAKey = (*KeySigner)(nil)

// NewPCRSigner creates a new PCR signer from Azure settings.
func NewPCRSigner(ctx context.Context, vaultURL, keyID, keyVersion string) (*KeySigner, error) {
	client, err := getKeysClient(vaultURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build Azure client: %w", err)
	}

	keyResponse, err := client.GetKey(ctx, keyID, keyVersion, &azkeys.GetKeyOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	if keyResponse.Key.Kty == nil {
		return nil, errors.New("key type is nil")
	}

	switch *keyResponse.Key.Kty { //nolint:exhaustive
	case azkeys.KeyTypeRSA, azkeys.KeyTypeRSAHSM:
		// expected, continue
	default:
		return nil, errors.New("key type is not RSA")
	}

	var publicKey rsa.PublicKey

	// N = modulus
	if len(keyResponse.Key.N) == 0 {
		return nil, errors.New("property N is empty")
	}

	publicKey.N = &big.Int{}
	publicKey.N.SetBytes(keyResponse.Key.N)

	// e = public exponent
	if len(keyResponse.Key.E) == 0 {
		return nil, errors.New("property e is empty")
	}

	publicKey.E = int(big.NewInt(0).SetBytes(keyResponse.Key.E).Uint64())

	return &KeySigner{
		keyName:    keyID,
		keyVersion: keyVersion,

		publicKey: &publicKey,
		client:    client,
	}, nil
}
