// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"
)

// Mode is a runtime mode.
type Mode int

const (
	// ModeCloud is the cloud runtime mode.
	ModeCloud Mode = iota
	// ModeContainer is the container runtime mode.
	ModeContainer
	// ModeMetal is the metal runtime mode.
	ModeMetal
)

const (
	cloud     = "cloud"
	container = "container"
	metal     = "metal"
)

// String returns the string representation of a Mode.
func (m Mode) String() string {
	return [...]string{cloud, container, metal}[m]
}

// ParseMode returns a `Mode` that matches the specified string.
func ParseMode(s string) (mod Mode, err error) {
	switch s {
	case cloud:
		mod = ModeCloud
	case container:
		mod = ModeContainer
	case metal:
		mod = ModeMetal
	default:
		return mod, fmt.Errorf("unknown runtime mode: %q", s)
	}

	return mod, nil
}
