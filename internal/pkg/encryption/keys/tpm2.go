// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"

	"github.com/siderolabs/go-blockdevice/blockdevice/encryption"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption/luks"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption/token"

	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
)

// TPMToken is the userdata stored in the partition token metadata.
type TPMToken struct {
	KeySlots          []int  `json:"keyslots"`
	SealedBlobPrivate []byte `json:"sealed_blob_private"`
	SealedBlobPublic  []byte `json:"sealed_blob_public"`
	PCRs              []int  `json:"pcrs"`
	Alg               string `json:"alg"`
	PolicyHash        []byte `json:"policy_hash"`
	KeyName           []byte `json:"key_name"`
}

// TPMKeyHandler seals token using TPM.
type TPMKeyHandler struct {
	KeyHandler
}

// NewTPMKeyHandler creates new TPMKeyHandler.
func NewTPMKeyHandler(key KeyHandler) (*TPMKeyHandler, error) {
	return &TPMKeyHandler{
		KeyHandler: key,
	}, nil
}

// NewKey implements Handler interface.
func (h *TPMKeyHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, err
	}

	resp, err := tpm2.Seal(key)
	if err != nil {
		return nil, nil, err
	}

	// explicitly clear key from memory since it's not needed anymore
	clear(key)

	token := &luks.Token[*TPMToken]{
		Type: TokenTypeTPM,
		UserData: &TPMToken{
			KeySlots:          []int{h.slot},
			SealedBlobPrivate: resp.SealedBlobPrivate,
			SealedBlobPublic:  resp.SealedBlobPublic,
			PCRs:              []int{secureboot.UKIPCR},
			Alg:               "sha256",
			PolicyHash:        resp.PolicyDigest,
			KeyName:           resp.KeyName,
		},
	}

	return encryption.NewKey(h.slot, []byte(base64.StdEncoding.EncodeToString(key))), token, nil
}

// GetKey implements Handler interface.
func (h *TPMKeyHandler) GetKey(ctx context.Context, t token.Token) (*encryption.Key, error) {
	token, ok := t.(*luks.Token[*TPMToken])
	if !ok {
		return nil, ErrTokenInvalid
	}

	sealed := tpm2.SealedResponse{
		SealedBlobPrivate: token.UserData.SealedBlobPrivate,
		SealedBlobPublic:  token.UserData.SealedBlobPublic,
		PolicyDigest:      token.UserData.PolicyHash,
		KeyName:           token.UserData.KeyName,
	}

	key, err := tpm2.Unseal(sealed)
	if err != nil {
		return nil, err
	}

	return encryption.NewKey(h.slot, []byte(base64.StdEncoding.EncodeToString(key))), nil
}
