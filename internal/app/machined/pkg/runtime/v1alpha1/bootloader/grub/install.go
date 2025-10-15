// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/internal/pkg/smbios"
	"github.com/siderolabs/talos/internal/pkg/uki"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	amd64 = "amd64"
	arm64 = "arm64"
)

// Install validates the grub configuration and writes it to the disk.
func (c *Config) Install(opts options.InstallOptions) (*options.InstallResult, error) {
	var installResult *options.InstallResult

	mountSpecs := []mount.Spec{
		{
			PartitionLabel: constants.BootPartitionLabel,
			FilesystemType: partition.FilesystemTypeXFS,
			MountTarget:    filepath.Join(opts.MountPrefix, constants.BootMountPoint),
		},
	}

	efiMountSpec := mount.Spec{
		PartitionLabel: constants.EFIPartitionLabel,
		FilesystemType: partition.FilesystemTypeVFAT,
		MountTarget:    filepath.Join(opts.MountPrefix, constants.EFIMountPoint),
	}

	// check if the EFI partition is present
	if err := mount.PartitionOp(
		opts.BootDisk,
		[]mount.Spec{efiMountSpec},
		func() error {
			return nil
		},
		[]blkid.ProbeOption{
			blkid.WithSkipLocking(true),
		},
		nil,
		nil,
		opts.BlkidInfo,
	); err == nil {
		c.installEFI = true
	}

	if c.installEFI {
		mountSpecs = append(mountSpecs, efiMountSpec)
	}

	err := mount.PartitionOp(
		opts.BootDisk,
		mountSpecs,
		func() error {
			var installErr error

			installResult, installErr = c.install(opts)

			return installErr
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

//nolint:gocyclo,cyclop
func (c *Config) install(opts options.InstallOptions) (*options.InstallResult, error) {
	if err := c.flip(); err != nil {
		return nil, err
	}

	cmdline := opts.Cmdline

	// if we have a kernel path, assume that the kernel and initramfs are available
	if _, err := os.Stat(opts.BootAssets.KernelPath); err == nil {
		if err := utils.CopyFiles(
			opts.Printf,
			utils.SourceDestination(
				opts.BootAssets.KernelPath,
				filepath.Join(opts.MountPrefix, constants.BootMountPoint, string(c.Default), constants.KernelAsset),
			),
			utils.SourceDestination(
				opts.BootAssets.InitramfsPath,
				filepath.Join(opts.MountPrefix, constants.BootMountPoint, string(c.Default), constants.InitramfsAsset),
			),
		); err != nil {
			return nil, err
		}

		if opts.GrubUseUKICmdline {
			return nil, fmt.Errorf("cannot use UKI cmdline when boot assets are not UKI")
		}
	} else {
		// if the kernel path does not exist, assume that the kernel and initramfs are in the UKI
		assetInfo, err := uki.Extract(opts.BootAssets.UKIPath)
		if err != nil {
			return nil, err
		}

		defer func() {
			if assetInfo.Closer != nil {
				assetInfo.Close() //nolint:errcheck
			}
		}()

		if err := utils.CopyReader(
			opts.Printf,
			utils.ReaderDestination(
				assetInfo.Kernel,
				filepath.Join(opts.MountPrefix, constants.BootMountPoint, string(c.Default), constants.KernelAsset),
			),
			utils.ReaderDestination(
				assetInfo.Initrd,
				filepath.Join(opts.MountPrefix, constants.BootMountPoint, string(c.Default), constants.InitramfsAsset),
			),
		); err != nil {
			return nil, err
		}

		if opts.GrubUseUKICmdline {
			cmdlineBytes, err := io.ReadAll(assetInfo.Cmdline)
			if err != nil {
				return nil, fmt.Errorf("failed to read cmdline from UKI: %w", err)
			}

			cmdline = string(cmdlineBytes)

			if extraCmdline, err := smbios.ReadOEMVariable(constants.SDStubCmdlineExtraOEMVar); err == nil {
				for _, extra := range extraCmdline {
					cmdline += " " + extra
				}
			}
		}
	}

	if err := c.Put(c.Default, cmdline, opts.Version); err != nil {
		return nil, err
	}

	if err := c.Write(filepath.Join(opts.MountPrefix, ConfigPath), opts.Printf); err != nil {
		return nil, err
	}

	var platforms []string

	switch opts.Arch {
	case amd64:
		if c.installEFI {
			platforms = append(platforms, "x86_64-efi")
		}

		platforms = append(platforms, "i386-pc")
	case arm64:
		platforms = []string{"arm64-efi"}
	}

	if runtime.GOARCH == amd64 && opts.Arch == amd64 && !opts.ImageMode {
		// let grub choose the platform automatically if not building an image
		platforms = []string{""}
	}

	for _, platform := range platforms {
		args := []string{
			"--boot-directory=" + filepath.Join(opts.MountPrefix, constants.BootMountPoint),
			"--removable",
		}

		if c.installEFI {
			args = append(args, "--efi-directory="+filepath.Join(opts.MountPrefix, constants.EFIMountPoint))
		}

		if opts.ImageMode {
			args = append(args, "--no-nvram")
		}

		if platform != "" {
			args = append(args, "--target="+platform)
		}

		args = append(args, opts.BootDisk)

		opts.Printf("executing: grub-install %s", strings.Join(args, " "))

		if _, err := cmd.Run("grub-install", args...); err != nil {
			return nil, fmt.Errorf("failed to install grub: %w", err)
		}
	}

	if opts.ExtraInstallStep != nil {
		if err := opts.ExtraInstallStep(); err != nil {
			return nil, err
		}
	}

	return &options.InstallResult{
		PreviousLabel: string(c.Fallback),
	}, nil
}
