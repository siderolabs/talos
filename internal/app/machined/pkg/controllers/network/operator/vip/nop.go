// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vip

import (
	"context"
)

// NopHandler does nothing.
type NopHandler struct{}

// Acquire implements Handler interface.
func (handler NopHandler) Acquire(ctx context.Context) error {
	return nil
}

// Release implements Handler interface.
func (handler NopHandler) Release(ctx context.Context) error {
	return nil
}
