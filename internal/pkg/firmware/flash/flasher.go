// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flash

import (
	"context"
	"fmt"
)

// LVFS release protocol IDs recognized by in-tree flashers.
const (
	ProtocolNVMe = "org.nvmexpress"
)

// Target describes the device to flash.
type Target struct {
	// DevPath is the block/character device path (e.g. /dev/nvme0n1).
	DevPath string
}

// Flasher stages a firmware payload on a device for activation at next reset.
type Flasher interface {
	Flash(ctx context.Context, target Target, payload []byte) error
}

var registry = map[string]Flasher{}

// Register makes a Flasher available for the given LVFS protocol ID.
//
// It panics if a Flasher is already registered for the protocol.
func Register(protocol string, f Flasher) {
	if _, ok := registry[protocol]; ok {
		panic(fmt.Sprintf("flasher already registered for protocol %q", protocol))
	}

	registry[protocol] = f
}

// For returns the Flasher registered for the given protocol.
func For(protocol string) (Flasher, bool) {
	f, ok := registry[protocol]

	return f, ok
}

// Supported reports whether a Flasher is registered for the given protocol.
func Supported(protocol string) bool {
	_, ok := registry[protocol]

	return ok
}
