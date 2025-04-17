// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/foxboron/go-uefi/efi"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/luks"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"

	"github.com/siderolabs/talos/internal/pkg/encryption/helpers"
	"github.com/siderolabs/talos/internal/pkg/secureboot/tpm2"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

	tpmLocker               helpers.TPMLockFunc
	checkSecurebootOnEnroll bool
}

// NewTPMKeyHandler creates new TPMKeyHandler.
func NewTPMKeyHandler(key KeyHandler, checkSecurebootOnEnroll bool, tpmLocker helpers.TPMLockFunc) (*TPMKeyHandler, error) {
	return &TPMKeyHandler{
		KeyHandler:              key,
		tpmLocker:               tpmLocker,
		checkSecurebootOnEnroll: checkSecurebootOnEnroll,
	}, nil
}

// NewKey implements Handler interface.
func (h *TPMKeyHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	if h.checkSecurebootOnEnroll {
		if !efi.GetSecureBoot() {
			return nil, nil, fmt.Errorf("failed to enroll the TPM2 key, as SecureBoot is disabled (and checkSecurebootOnEnroll is enabled)")
		}

		if efi.GetSetupMode() {
			return nil, nil, fmt.Errorf("failed to enroll the TPM2 key, as the system is in SecureBoot setup mode (and checkSecurebootOnEnroll is enabled)")
		}
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, nil, err
	}

	var resp *tpm2.SealedResponse

	if err := h.tpmLocker(ctx, func() error {
		var err error

		resp, err = tpm2.Seal(key)

		return err
	}); err != nil {
		return nil, nil, err
	}

	token := &luks.Token[*TPMToken]{
		Type: TokenTypeTPM,
		UserData: &TPMToken{
			KeySlots:          []int{h.slot},
			SealedBlobPrivate: resp.SealedBlobPrivate,
			SealedBlobPublic:  resp.SealedBlobPublic,
			PCRs:              []int{constants.UKIPCR},
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

	var key []byte

	if err := h.tpmLocker(ctx, func() error {
		var err error

		key, err = tpm2.Unseal(sealed)

		return err
	}); err != nil {
		return nil, err
	}

	return encryption.NewKey(h.slot, []byte(base64.StdEncoding.EncodeToString(key))), nil
}
