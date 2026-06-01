// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"
	"fmt"
	"slices"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"

	"github.com/siderolabs/talos/internal/pkg/encryption/helpers"
)

// SaltedHandler is a key handler wrapper that salts the key with a provided random salt.
type SaltedHandler struct {
	wrapped    Handler
	saltGetter helpers.SaltGetter
}

// NewSaltedHandler creates a new handler that wraps the provided key handler and uses the provided salt getter.
func NewSaltedHandler(wrapped Handler, saltGetter helpers.SaltGetter) Handler {
	return &SaltedHandler{
		wrapped:    wrapped,
		saltGetter: saltGetter,
	}
}

// NewKey implements the keys.Handler interface.
func (k *SaltedHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	key, token, err := k.wrapped.NewKey(ctx)
	if err != nil {
		return key, token, err
	}

	salt, err := k.saltGetter(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get disk encryption key salt: %w", err)
	}

	key.Value = slices.Concat(key.Value, salt)

	return key, token, nil
}

// GetKey implements the keys.Handler interface.
func (k *SaltedHandler) GetKey(ctx context.Context, token token.Token) (*encryption.Key, error) {
	key, err := k.wrapped.GetKey(ctx, token)
	if err != nil {
		return key, err
	}

	salt, err := k.saltGetter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk encryption key salt: %w", err)
	}

	key.Value = slices.Concat(key.Value, salt)

	return key, nil
}

// Slot implements the keys.Handler interface.
func (k *SaltedHandler) Slot() int {
	return k.wrapped.Slot()
}
