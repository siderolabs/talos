// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"
)

// StaticKeyHandler just handles the static key value all the time.
type StaticKeyHandler struct {
	KeyHandler
	data []byte
}

// NewStaticKeyHandler creates new EphemeralKeyHandler.
func NewStaticKeyHandler(key KeyHandler, data []byte) *StaticKeyHandler {
	return &StaticKeyHandler{
		KeyHandler: key,
		data:       data,
	}
}

// NewKey implements Handler interface.
func (h *StaticKeyHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	k, err := h.GetKey(ctx, nil)

	return k, nil, err
}

// GetKey implements Handler interface.
func (h *StaticKeyHandler) GetKey(context.Context, token.Token) (*encryption.Key, error) {
	return encryption.NewKey(h.slot, h.data), nil
}
