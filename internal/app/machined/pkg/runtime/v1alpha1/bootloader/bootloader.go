// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

import (
	"os"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Bootloader describes a bootloader.
type Bootloader interface {
	// Install the bootloader.
	//
	// Install mounts the partitions as required.
	Install(options options.InstallOptions) (*options.InstallResult, error)
	// Revert reverts the bootloader entry to the previous state.
	//
	// Revert mounts the partitions as required.
	Revert(disk string) error
	// RequiredPartitions returns the required partitions for the bootloader.
	RequiredPartitions() []partition.Options

	// KexecLoad does a kexec_file_load using the current entry of the bootloader.
	KexecLoad(r runtime.Runtime, disk string) error
}

// Probe checks if any supported bootloaders are installed.
//
// Returns nil if it cannot detect any supported bootloader.
func Probe(disk string, options options.ProbeOptions) (Bootloader, error) {
	grubBootloader, err := grub.Probe(disk, options)
	if err != nil {
		return nil, err
	}

	if grubBootloader != nil {
		return grubBootloader, nil
	}

	sdbootBootloader, err := sdboot.Probe(disk, options)
	if err != nil {
		return nil, err
	}

	if sdbootBootloader != nil {
		return sdbootBootloader, nil
	}

	return nil, os.ErrNotExist
}

// NewAuto returns a new bootloader based on auto-detection.
func NewAuto() Bootloader {
	if sdboot.IsBootedUsingSDBoot() {
		return sdboot.New()
	}

	return grub.NewConfig()
}

// New returns a new bootloader based on the secureboot flag.
func New(secureboot bool, talosVersion string) Bootloader {
	if secureboot {
		return sdboot.New()
	}

	g := grub.NewConfig()
	g.AddResetOption = quirks.New(talosVersion).SupportsResetGRUBOption()

	return g
}
