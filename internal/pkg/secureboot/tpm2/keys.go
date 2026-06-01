// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tpm2 provides TPM2.0 related functionality helpers.
package tpm2

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-tpm/tpm2"
)

// ParsePCRSigningPubKey parses a PEM encoded RSA public key.
func ParsePCRSigningPubKey(file string) (*rsa.PublicKey, error) {
	pcrSigningPubKey, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read pcr signing public key: %v", err)
	}

	block, _ := pem.Decode(pcrSigningPubKey)
	if block == nil {
		return nil, errors.New("failed to decode pcr signing public key")
	}

	// parse rsa public key
	tpm2PubKeyAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	tpm2PubKey, ok := tpm2PubKeyAny.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("failed to cast pcr signing public key to rsa")
	}

	return tpm2PubKey, nil
}

// RSAPubKeyTemplate returns a TPM2.0 public key template for RSA keys.
func RSAPubKeyTemplate(bitlen, exponent int, modulus []byte) tpm2.TPMTPublic {
	return tpm2.TPMTPublic{
		Type:    tpm2.TPMAlgRSA,
		NameAlg: tpm2.TPMAlgSHA256,
		ObjectAttributes: tpm2.TPMAObject{
			Decrypt:      true,
			SignEncrypt:  true,
			UserWithAuth: true,
		},
		Parameters: tpm2.NewTPMUPublicParms(tpm2.TPMAlgRSA, &tpm2.TPMSRSAParms{
			Symmetric: tpm2.TPMTSymDefObject{
				Algorithm: tpm2.TPMAlgNull,
				Mode:      tpm2.NewTPMUSymMode(tpm2.TPMAlgRSA, tpm2.TPMAlgNull),
			},
			Scheme: tpm2.TPMTRSAScheme{
				Scheme: tpm2.TPMAlgNull,
				Details: tpm2.NewTPMUAsymScheme(tpm2.TPMAlgRSA, &tpm2.TPMSSigSchemeRSASSA{
					HashAlg: tpm2.TPMAlgNull,
				}),
			},
			KeyBits:  tpm2.TPMKeyBits(bitlen),
			Exponent: uint32(exponent),
		}),
		Unique: tpm2.NewTPMUPublicID(tpm2.TPMAlgRSA, &tpm2.TPM2BPublicKeyRSA{
			Buffer: modulus,
		}),
	}
}
