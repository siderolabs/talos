// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vip contains implementations of specific methods to acquire/release virtual IPs.
package vip

import "context"

// Handler implements custom actions to manage virtual IP assignment.
type Handler interface {
	Acquire(ctx context.Context) error
	Release(ctx context.Context) error
}
