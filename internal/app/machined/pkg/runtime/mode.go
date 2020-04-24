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
	// Cloud is the cloud runtime mode.
	Cloud Mode = iota
	// Container is the container runtime mode.
	Container
	// Interactive is the interactive runtime mode.
	Interactive
	// Metal is the metal runtime mode.
	Metal
)

const (
	cloud       = "cloud"
	container   = "container"
	interactive = "interactive"
	metal       = "metal"
)

// String returns the string representation of a Mode.
func (m Mode) String() string {
	return [...]string{cloud, container, interactive, metal}[m]
}

// ParseMode returns a `Mode` that matches the specified string.
func ParseMode(s string) (mod Mode, err error) {
	switch s {
	case cloud:
		mod = Cloud
	case container:
		mod = Container
	case interactive:
		mod = Interactive
	case metal:
		mod = Metal
	default:
		return mod, fmt.Errorf("unknown runtime mode: %q", s)
	}

	return mod, nil
}
