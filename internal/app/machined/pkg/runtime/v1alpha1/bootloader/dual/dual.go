// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dual provides dual-boot bootloader implementation.
package dual

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/internal/pkg/uki"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Config describes a dual-boot bootloader.
// this is a dummy implementation of the bootloader interface
// allowing to install GRUB for BIOS and sd-boot for UEFI
// so we only care about `RequiredPartitions()` and `Install()`.
type Config struct{}

// New creates a new bootloader.
func New() *Config {
	return &Config{}
}

// Install installs the bootloader.
func (c *Config) Install(opts options.InstallOptions) (*options.InstallResult, error) {
	if !opts.ImageMode {
		return nil, fmt.Errorf("dual-boot bootloader is only supported in image mode")
	}

	var installResult *options.InstallResult

	err := mount.PartitionOp(
		opts.BootDisk,
		[]mount.Spec{
			{
				PartitionLabel: constants.BootPartitionLabel,
				FilesystemType: partition.FilesystemTypeXFS,
				MountTarget:    filepath.Join(opts.MountPrefix, constants.BootMountPoint),
			},
			{
				PartitionLabel: constants.EFIPartitionLabel,
				FilesystemType: partition.FilesystemTypeVFAT,
				MountTarget:    filepath.Join(opts.MountPrefix, constants.EFIMountPoint),
			},
		},
		func() error {
			if err := c.installGrub(opts); err != nil {
				return err
			}

			if err := c.installSDBoot(opts); err != nil {
				return err
			}

			if opts.ExtraInstallStep != nil {
				if err := opts.ExtraInstallStep(); err != nil {
					return err
				}
			}

			return nil
		},
		[]blkid.ProbeOption{
			// installation happens with locked blockdevice
			blkid.WithSkipLocking(true),
		},
		nil,
		nil,
		opts.BlkidInfo,
	)

	return installResult, err
}

// Revert is not implemented.
func (c *Config) Revert(disk string) error {
	return fmt.Errorf("not implemented")
}

// RequiredPartitions returns the list of partitions required by the bootloader.
func (c *Config) RequiredPartitions(quirk quirks.Quirks) []partition.Options {
	return []partition.Options{
		partition.NewPartitionOptions(constants.EFIPartitionLabel, true, quirk),
		partition.NewPartitionOptions(constants.BIOSGrubPartitionLabel, false, quirk),
		partition.NewPartitionOptions(constants.BootPartitionLabel, false, quirk),
	}
}

// KexecLoad is not implemented.
func (c *Config) KexecLoad(r runtime.Runtime, disk string) error {
	return fmt.Errorf("not implemented")
}

func (c *Config) installGrub(opts options.InstallOptions) error {
	assetInfo, err := uki.Extract(opts.BootAssets.UKIPath)
	if err != nil {
		return err
	}

	defer func() {
		if assetInfo.Closer != nil {
			assetInfo.Close() //nolint:errcheck
		}
	}()

	grubConfig := grub.NewConfig()

	if err := utils.CopyReader(
		opts.Printf,
		utils.ReaderDestination(
			assetInfo.Kernel,
			filepath.Join(opts.MountPrefix, constants.BootMountPoint, string(grubConfig.Default), constants.KernelAsset),
		),
		utils.ReaderDestination(
			assetInfo.Initrd,
			filepath.Join(opts.MountPrefix, constants.BootMountPoint, string(grubConfig.Default), constants.InitramfsAsset),
		),
	); err != nil {
		return err
	}

	if err := grubConfig.Put(grubConfig.Default, opts.Cmdline, opts.Version); err != nil {
		return err
	}

	if err := grubConfig.Write(filepath.Join(opts.MountPrefix, grub.ConfigPath), opts.Printf); err != nil {
		return err
	}

	args := []string{
		"--boot-directory=" + filepath.Join(opts.MountPrefix, constants.BootMountPoint),
		"--removable",
		"--no-nvram",
		"--target=i386-pc",
	}

	args = append(args, opts.BootDisk)

	opts.Printf("executing: grub-install %s", strings.Join(args, " "))

	if _, err := cmd.Run("grub-install", args...); err != nil {
		return fmt.Errorf("failed to install grub: %w", err)
	}

	return nil
}

func (c *Config) installSDBoot(opts options.InstallOptions) error {
	var sdbootFilename string

	switch opts.Arch {
	case "amd64":
		sdbootFilename = "BOOTX64.efi"
	case "arm64":
		sdbootFilename = "BOOTAA64.efi"
	default:
		return fmt.Errorf("unsupported architecture: %s", opts.Arch)
	}

	// writing UKI by version-based filename here
	ukiPath := fmt.Sprintf("%s-%s.efi", "Talos", opts.Version)

	if err := utils.CopyFiles(
		opts.Printf,
		utils.SourceDestination(
			opts.BootAssets.UKIPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", ukiPath),
		),
		utils.SourceDestination(
			opts.BootAssets.SDBootPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "boot", sdbootFilename),
		),
	); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader"), 0o755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader", "loader.conf"), sdboot.LoaderConfBytes, 0o644); err != nil {
		return err
	}

	return nil
}
