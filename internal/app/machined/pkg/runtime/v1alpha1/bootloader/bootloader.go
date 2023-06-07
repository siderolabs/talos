// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

import "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"

// Bootloader describes a bootloader.
type Bootloader interface {
	// Install installs the bootloader
	Install(bootDisk, arch, cmdline string) error
	// Flip flips the bootloader entry to the next state.
	Flip() error
	// Revert reverts the bootloader entry to the previous state.
	Revert() error
	// NextLabel returns the next bootloader label.
	NextLabel() string
	// PreviousLabel returns the previous bootloader label.
	PreviousLabel() string
	// Installed returns true if the bootloader is installed.
	Installed() bool
}

// Probe checks if any supported bootloaders are installed.
// Returns nil if it cannot detect any supported bootloader.
func Probe(skipProbe bool) (Bootloader, error) {
	// skipProbe skips bootloader probing.
	if skipProbe {
		return nil, nil
	}

	bootloader, err := grub.Probe()
	if err != nil {
		return nil, err
	}

	return bootloader, nil
}
