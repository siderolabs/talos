// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"encoding/binary"
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

	return &options.InstallResult{
		PreviousLabel: string(c.Fallback),
	}, err
}

func (c *Config) generateGrubImage(opts options.InstallOptions) error {
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
		filepath.Join(opts.MountPrefix, efiFile),
	))

	if err := utils.CopyFiles(
		opts.Printf,
		copyInstructions...,
	); err != nil {
		return fmt.Errorf("failed to copy grub generated img files: %w", err)
	}

	if opts.Arch == amd64 {
		if err := c.patchGrubBlocklists(opts); err != nil {
			return fmt.Errorf("failed to patch GRUB blocklists: %w", err)
		}
	}

	return nil
}

//nolint:gocyclo
func (c *Config) generateAssets(opts options.InstallOptions) error {
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
		return c.generateGrubImage(opts)
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

// patchGrubBlocklists patches the GRUB boot.img and core.img with blocklist information
// for GPT+BIOS boot without requiring grub-install or loop devices.
//
// References (GRUB source tree inside orb VM):
// - grub-core/boot/i386/pc/boot.S: defines GRUB_BOOT_MACHINE_KERNEL_SECTOR and MBR code
// - include/grub/i386/pc/boot.h: GRUB_BOOT_MACHINE_KERNEL_SECTOR == 0x5c
// - util/setup.c: write_rootdev() patches boot.img fields and writes sector in LE64
// - core image embedded blocklist continuation at core.img offset 0x1F4.
func (c *Config) patchGrubBlocklists(opts options.InstallOptions) error {
	// Talos partition layout (GPT): EFI (gpt1, 100MiB), BIOS (gpt2, 1MiB), BOOT (gpt3)
	// BIOS boot partition starts immediately after the EFI partition.
	const (
		firstPartitionStart       = 2048                    // sector 2048 (1MiB)
		efiPartitionSectors       = 100 * 1024 * 1024 / 512 // 100MiB in sectors
		biosBootStartSector       = firstPartitionStart + efiPartitionSectors
		bootImgKernelSectorOffset = 0x5c  // include/grub/i386/pc/boot.h (GRUB_BOOT_MACHINE_KERNEL_SECTOR)
		bootImgJumpOffset         = 0x66  // patched to NOP NOP (0x90 0x90) by grub-install on GPT
		coreImgBlocklistOffset    = 0x1f4 // embedded blocklist continuation inside core.img
	)

	bootImgPath := filepath.Join(opts.MountPrefix, "boot.img")
	coreImgPath := filepath.Join(opts.MountPrefix, "core.img")

	bootImg, err := os.ReadFile(bootImgPath)
	if err != nil {
		return fmt.Errorf("failed to read boot.img: %w", err)
	}

	// Patch 1: tell boot.img where to find core.img (LE64 sector number at 0x5C)
	binary.LittleEndian.PutUint64(bootImg[bootImgKernelSectorOffset:], uint64(biosBootStartSector))
	// Patch 2: NOP the short jump at 0x66 for GPT installs (matches grub-install behavior)
	bootImg[bootImgJumpOffset] = 0x90
	bootImg[bootImgJumpOffset+1] = 0x90

	if err := os.WriteFile(bootImgPath, bootImg, 0o644); err != nil {
		return fmt.Errorf("failed to write patched boot.img: %w", err)
	}

	coreImg, err := os.ReadFile(coreImgPath)
	if err != nil {
		return fmt.Errorf("failed to read core.img: %w", err)
	}

	// Patch 3: core.img embedded blocklist continuation (LE64) points to start+1
	cont := uint64(biosBootStartSector + 1)
	binary.LittleEndian.PutUint64(coreImg[coreImgBlocklistOffset:], cont)

	if err := os.WriteFile(coreImgPath, coreImg, 0o644); err != nil {
		return fmt.Errorf("failed to write patched core.img: %w", err)
	}

	opts.Printf("patched GRUB blocklists: boot.img->%d, core.img next->%d", biosBootStartSector, biosBootStartSector+1)

	return nil
}
