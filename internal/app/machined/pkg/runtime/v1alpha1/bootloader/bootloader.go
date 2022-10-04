// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

// Bootloader describes a bootloader.
type Bootloader interface {
	// Install installs the bootloader
	Install(bootDisk, arch string) error
}
