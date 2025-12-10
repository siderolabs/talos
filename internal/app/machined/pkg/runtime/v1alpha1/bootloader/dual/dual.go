// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dual provides dual-boot bootloader implementation.
package dual

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Config describes a dual-boot bootloader.
// this is a dummy implementation of the bootloader interface
// allowing to install GRUB for BIOS and sd-boot for UEFI
// so we only care about `GenerateAssets()`.
type Config struct{}

// New creates a new bootloader.
func New() *Config {
	return &Config{}
}

// GenerateAssets generates the dual-boot bootloader assets and returns the partition options with source directory set.
func (c *Config) GenerateAssets(efiAssetsPath string, opts options.InstallOptions) ([]partition.Options, error) {
	if opts.Arch == "arm64" {
		return nil, fmt.Errorf("dual-boot bootloader is not supported on arm64 architecture, either GRUB or sd-boot must be used")
	}

	// here we'll use the grub and sd-boot GenerateAssets logic
	// and remove the grub `EFI` directory after we're done
	if _, err := grub.NewConfig().GenerateAssets(efiAssetsPath, opts); err != nil {
		return nil, fmt.Errorf("failed to install GRUB bootloader: %w", err)
	}

	if err := os.RemoveAll(filepath.Join(opts.MountPrefix, efiAssetsPath)); err != nil {
		return nil, fmt.Errorf("failed to cleanup GRUB EFI assets directory: %w", err)
	}

	if _, err := sdboot.New().GenerateAssets(efiAssetsPath, opts); err != nil {
		return nil, fmt.Errorf("failed to generate sd-boot assets: %w", err)
	}

	quirk := quirks.New(opts.Version)

	partitionOptions := []partition.Options{
		partition.NewPartitionOptions(
			true,
			quirk,
			partition.WithLabel(constants.EFIPartitionLabel),
			partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, efiAssetsPath)),
		),
		partition.NewPartitionOptions(false, quirk, partition.WithLabel(constants.BIOSGrubPartitionLabel)),
		partition.NewPartitionOptions(
			false,
			quirk,
			partition.WithLabel(constants.BootPartitionLabel),
			partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, constants.BootMountPoint)),
		),
	}

	if opts.ImageMode {
		partitionOptions = xslices.Map(partitionOptions, func(o partition.Options) partition.Options {
			o.Reproducible = true

			return o
		})
	}

	return partitionOptions, nil
}

// Install installs the bootloader.
func (c *Config) Install(opts options.InstallOptions) (*options.InstallResult, error) {
	return nil, fmt.Errorf("dual-boot bootloader is only supported in image mode, installation is not implemented")
}

// Upgrade is not implemented since dual-boot is only supported in image mode.
func (c *Config) Upgrade(opts options.InstallOptions) (*options.InstallResult, error) {
	return nil, fmt.Errorf("dual-boot bootloader is only supported in image mode, upgrade is not implemented")
}

// Revert is not implemented.
func (c *Config) Revert(disk string) error {
	return fmt.Errorf("dual-boot bootloader is only supported in image mode, revert is not implemented")
}

// KexecLoad is not implemented.
func (c *Config) KexecLoad(r runtime.Runtime, disk string) error {
	return fmt.Errorf("dual-boot bootloader is only supported in image mode, kexec load is not implemented")
}
