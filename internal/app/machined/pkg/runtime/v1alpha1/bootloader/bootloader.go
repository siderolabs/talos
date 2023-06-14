// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

import (
	"os"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
)

// Bootloader describes a bootloader.
type Bootloader interface {
	// Install installs the bootloader
	Install(bootDisk, arch, cmdline string) error
	// Revert reverts the bootloader entry to the previous state.
	Revert() error
	// PreviousLabel returns the previous bootloader label.
	PreviousLabel() string
	// UEFIBoot returns true if the bootloader is UEFI-only.
	UEFIBoot() bool
}

// Probe checks if any supported bootloaders are installed.
//
// If 'disk' is empty, it will probe all disks.
// Returns nil if it cannot detect any supported bootloader.
func Probe(disk string) (Bootloader, error) {
	grubBootloader, err := grub.Probe(disk)
	if err != nil {
		return nil, err
	}

	if grubBootloader != nil {
		return grubBootloader, nil
	}

	sdbootBootloader, err := sdboot.Probe(disk)
	if err != nil {
		return nil, err
	}

	if sdbootBootloader != nil {
		return sdbootBootloader, nil
	}

	return nil, os.ErrNotExist
}

// New returns a new bootloader.
func New() (Bootloader, error) {
	// TODO: there should be a way to force sd-boot/GRUB based on installer args,
	//       to build a disk image with specified bootloader.
	if sdboot.IsBootedUsingSDBoot() {
		return sdboot.New(), nil
	}

	return grub.NewConfig(), nil
}
