// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

import "fmt"

// BootLabel represents a boot label, e.g. A or B.
type BootLabel string

const (
	// BootA is a bootloader label.
	BootA BootLabel = "A"
	// BootB is a bootloader label.
	BootB BootLabel = "B"
	// BootReset is a bootloader label.
	BootReset BootLabel = "Reset"
)

// FlipBootLabel flips the boot label.
func FlipBootLabel(e BootLabel) (BootLabel, error) {
	switch e {
	case BootA:
		return BootB, nil
	case BootB:
		return BootA, nil
	case BootReset:
		fallthrough
	default:
		return "", fmt.Errorf("invalid entry: %s", e)
	}
}
