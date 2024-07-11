// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package keys contains various encryption KeyHandler implementations.
package keys

import (
	"context"
	"errors"
	"fmt"

	"github.com/siderolabs/go-blockdevice/blockdevice/encryption"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption/token"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

var errNoSystemInfoGetter = errors.New("the UUID getter is not set")

// NewHandler key using provided config.
func NewHandler(cfg config.EncryptionKey, options ...KeyOption) (Handler, error) {
	opts, err := NewDefaultOptions(options)
	if err != nil {
		return nil, err
	}

	key := KeyHandler{slot: cfg.Slot()}

	switch {
	case cfg.Static() != nil:
		k := cfg.Static().Key()
		if k == nil {
			return nil, errors.New("static key must have key data defined")
		}

		return NewStaticKeyHandler(key, k), nil
	case cfg.NodeID() != nil:
		if opts.GetSystemInformation == nil {
			return nil, fmt.Errorf("failed to create nodeUUID key handler at slot %d: %w", cfg.Slot(), errNoSystemInfoGetter)
		}

		return NewNodeIDKeyHandler(key, opts.PartitionLabel, opts.GetSystemInformation), nil
	case cfg.KMS() != nil:
		if opts.GetSystemInformation == nil {
			return nil, fmt.Errorf("failed to create KMS key handler at slot %d: %w", cfg.Slot(), errNoSystemInfoGetter)
		}

		return NewKMSKeyHandler(key, cfg.KMS().Endpoint(), opts.GetSystemInformation)
	case cfg.TPM() != nil:
		return NewTPMKeyHandler(key, cfg.TPM().CheckSecurebootOnEnroll())
	}

	return nil, errors.New("malformed config: no key handler can be created")
}

// Handler manages key lifecycle.
type Handler interface {
	NewKey(context.Context) (*encryption.Key, token.Token, error)
	GetKey(context.Context, token.Token) (*encryption.Key, error)
	Slot() int
}

// KeyHandler is the base class for all key handlers.
type KeyHandler struct {
	slot int
}

// Slot implements Handler interface.
func (k *KeyHandler) Slot() int {
	return k.slot
}

// ErrTokenInvalid is returned by the keys handler if the supplied token is not valid.
var ErrTokenInvalid = errors.New("invalid token")
