// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

import (
	"context"
	"os"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Bootloader describes a bootloader.
type Bootloader interface {
	// Install installs the bootloader
	Install(options options.InstallOptions) error
	// Revert reverts the bootloader entry to the previous state.
	Revert(ctx context.Context) error
	// PreviousLabel returns the previous bootloader label.
	PreviousLabel() string
	// UEFIBoot returns true if the bootloader is UEFI-only.
	UEFIBoot() bool
}

// Probe checks if any supported bootloaders are installed.
//
// If 'disk' is empty, it will probe all disks.
// Returns nil if it cannot detect any supported bootloader.
func Probe(ctx context.Context, disk string) (Bootloader, error) {
	grubBootloader, err := grub.Probe(ctx, disk)
	if err != nil {
		return nil, err
	}

	if grubBootloader != nil {
		return grubBootloader, nil
	}

	sdbootBootloader, err := sdboot.Probe(ctx, disk)
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
