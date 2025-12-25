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
	"slices"
	"strings"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-cmd/pkg/cmd"

	bootloaderutils "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/efiutils"
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

	var efiFound bool

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
		efiFound = true
	}

	if efiFound {
		mountSpecs = append(mountSpecs, efiMountSpec)
	}

	err := mount.PartitionOp(
		opts.BootDisk,
		mountSpecs,
		func() error {
			if err := c.runGrubInstall(opts, efiFound); err != nil {
				return err
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

	return &options.InstallResult{
		PreviousLabel: string(c.Fallback),
	}, err
}

func (c *Config) generateGrubImage(opts options.InstallOptions, efiAssetsPath string) error {
	var copyInstructions []utils.CopyInstruction

	grubSourceDirectory := "/usr/lib/grub"

	grubModules := []string{
		"part_gpt",
		"ext2",
		"fat",
		"xfs",
		"normal",
		"configfile",
		"linux",
		"boot",
		"search",
		"search_fs_uuid",
		"search_fs_file",
		"ls",
		"cat",
		"echo",
		"test",
		"help",
		"reboot",
		"halt",
		"all_video",
	}

	if opts.Arch == "amd64" {
		grub32Modules := []string{
			"biosdisk",
			"part_msdos",
		}

		args := []string{
			"--format",
			"i386-pc",
			"--output",
			filepath.Join(opts.MountPrefix, "core.img"),
			"--prefix",
			"(hd0,gpt3)/grub",
		}

		args = append(args, slices.Concat(grubModules, grub32Modules)...)

		if _, err := cmd.Run(
			"grub-mkimage",
			args...,
		); err != nil {
			return fmt.Errorf("failed to generate grub core image: %w", err)
		}

		copyInstructions = append(copyInstructions, utils.SourceDestination(
			filepath.Join(grubSourceDirectory, "i386-pc", "boot.img"),
			filepath.Join(opts.MountPrefix, "boot.img"),
		))
	}

	grubEFIPath := filepath.Join(opts.MountPrefix, "grub-efi.img")

	var (
		platform string
		prefix   string
	)

	switch opts.Arch {
	case "amd64":
		platform = "x86_64-efi"
		prefix = "(hd0,gpt3)/grub" // EFI, BIOS, BOOT
	case "arm64":
		platform = "arm64-efi"
		prefix = "(hd0,gpt2)/grub" // EFI, BOOT
	default:
		return fmt.Errorf("unsupported architecture for grub image: %s", opts.Arch)
	}

	args := []string{
		"--format",
		platform,
		"--output",
		grubEFIPath,
		"--prefix",
		prefix,
		"--compression",
		"xz",
	}
	args = append(args, grubModules...)

	if _, err := cmd.Run(
		"grub-mkimage",
		args...,
	); err != nil {
		return fmt.Errorf("failed to generate grub efi image: %w", err)
	}

	efiFile, err := bootloaderutils.Name(opts.Arch)
	if err != nil {
		return err
	}

	copyInstructions = append(copyInstructions, utils.SourceDestination(
		grubEFIPath,
		filepath.Join(opts.MountPrefix, efiAssetsPath, efiFile),
	))

	if err := utils.CopyFiles(
		opts.Printf,
		copyInstructions...,
	); err != nil {
		return fmt.Errorf("failed to copy grub generated img files: %w", err)
	}

	return nil
}

//nolint:gocyclo
func (c *Config) generateAssets(opts options.InstallOptions, efiAssetsPath string) error {
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
			return err
		}

		if opts.GrubUseUKICmdline {
			return fmt.Errorf("cannot use UKI cmdline when boot assets are not UKI")
		}
	} else {
		// if the kernel path does not exist, assume that the kernel and initramfs are in the UKI
		assetInfo, err := uki.Extract(opts.BootAssets.UKIPath)
		if err != nil {
			return err
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
			return err
		}

		if opts.GrubUseUKICmdline {
			cmdlineBytes, err := io.ReadAll(assetInfo.Cmdline)
			if err != nil {
				return fmt.Errorf("failed to read cmdline from UKI: %w", err)
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
		return err
	}

	if err := c.Write(filepath.Join(opts.MountPrefix, ConfigPath), opts.Printf); err != nil {
		return err
	}

	if opts.ImageMode {
		return c.generateGrubImage(opts, efiAssetsPath)
	}

	return nil
}

//nolint:gocyclo
func (c *Config) runGrubInstall(opts options.InstallOptions, efiMode bool) error {
	var platforms []string

	switch opts.Arch {
	case amd64:
		if efiMode {
			platforms = append(platforms, "x86_64-efi")
		}

		platforms = append(platforms, "i386-pc")
	case arm64:
		platforms = []string{"arm64-efi"}
	}

	if runtime.GOARCH == amd64 && opts.Arch == amd64 {
		// let grub choose the platform automatically if not building an image
		platforms = []string{""}
	}

	for _, platform := range platforms {
		args := []string{
			"--boot-directory=" + filepath.Join(opts.MountPrefix, constants.BootMountPoint),
			"--removable",
		}

		if efiMode {
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
			return fmt.Errorf("failed to install grub: %w", err)
		}
	}

	return nil
}
