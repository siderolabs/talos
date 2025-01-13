// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pcr contains code that handles PCR operations.
package pcr

import (
	"crypto"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Signature returns the hashed signature digest and base64 encoded signature.
type Signature struct {
	Digest          string
	SignatureBase64 string
}

// Sign the digest using specified hash and key.
func Sign(digest []byte, hash crypto.Hash, key crypto.Signer) (*Signature, error) {
	digestToHash := hash.New()
	digestToHash.Write(digest)
	digestHashed := digestToHash.Sum(nil)

	// sign policy digest
	signedData, err := key.Sign(nil, digestHashed, hash)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %v", err)
	}

	return &Signature{
		Digest:          hex.EncodeToString(digest),
		SignatureBase64: base64.StdEncoding.EncodeToString(signedData),
	}, nil
}
