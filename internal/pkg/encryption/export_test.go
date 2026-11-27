// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encryption

import (
	"context"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
)

// NewTestHandler creates a Handler with the given provider and key handlers, bypassing the config parsing.
func NewTestHandler(provider encryption.Provider, handlers []keys.Handler) *Handler {
	return &Handler{
		encryptionProvider: provider,
		keyHandlers:        handlers,
	}
}

// SyncKeys exposes syncKeys for tests.
func (h *Handler) SyncKeys(ctx context.Context, logger *zap.Logger, path string, k *encryption.Key) ([]string, error) {
	return h.syncKeys(ctx, logger, path, k)
}
