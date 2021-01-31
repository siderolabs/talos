// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package keys contains various encryption KeyHandler implementations.
package keys

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// NewHandler creates a new key handler depending on key handler kind.
func NewHandler(key config.EncryptionKey) (Handler, error) {
	switch {
	case key.Static() != nil:
		k := key.Static().Key()
		if k == nil {
			return nil, fmt.Errorf("static key must have key data defined")
		}

		return NewStaticKeyHandler(k)
	case key.NodeID() != nil:
		return NewNodeIDKeyHandler()
	}

	return nil, fmt.Errorf("failed to create key handler: malformed config")
}

// Handler represents an interface for fetching encryption keys.
type Handler interface {
	GetKey(options ...KeyOption) ([]byte, error)
}
