// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootloader provides bootloader implementation.
package bootloader

import (
	"fmt"
	"os"

	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/dual"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Bootloader describes a bootloader.
type Bootloader interface {
	GenerateAssets(options options.InstallOptions) ([]partition.Options, error)
	// Install the bootloader.
	//
	// Install mounts the partitions as required.
	Install(options options.InstallOptions) (*options.InstallResult, error)
	// Upgrade upgrades the bootloader installation.
	//
	// Upgrade mounts the partitions as required.
	Upgrade(options options.InstallOptions) (*options.InstallResult, error)
	// Revert reverts the bootloader entry to the previous state.
	//
	// Revert mounts the partitions as required.
	Revert(disk string) error

	// KexecLoad does a kexec_file_load using the current entry of the bootloader.
	KexecLoad(r runtime.Runtime, disk string) error
}

// Probe checks if any supported bootloaders are installed.
//
// Returns nil if it cannot detect any supported bootloader.
func Probe(disk string, options options.ProbeOptions) (Bootloader, error) {
	options.Logf("probing bootloader on %q", disk)

	grubBootloader, err := grub.Probe(disk, options)
	if err != nil {
		return nil, err
	}

	if grubBootloader != nil {
		options.Logf("found GRUB bootloader on %q", disk)

		return grubBootloader, nil
	}

	sdbootBootloader, err := sdboot.Probe(disk, options)
	if err != nil {
		return nil, err
	}

	if sdbootBootloader != nil {
		options.Logf("found sd-boot bootloader on %q", disk)

		return sdbootBootloader, nil
	}

	return nil, os.ErrNotExist
}

// NewAuto returns a new bootloader based on auto-detection.
func NewAuto() Bootloader {
	if sdboot.IsUEFIBoot() {
		return sdboot.New()
	}

	return grub.NewConfig()
}

// New returns a new bootloader based on the secureboot flag and architecture.
func New(bootloader, talosVersion, arch string) (Bootloader, error) {
	switch bootloader {
	case profile.BootLoaderKindGrub.String():
		g := grub.NewConfig()
		g.AddResetOption = quirks.New(talosVersion).SupportsResetGRUBOption()

		return g, nil
	case profile.BootLoaderKindSDBoot.String():
		return sdboot.New(), nil
	case profile.BootLoaderKindDualBoot.String():
		return dual.New(), nil
	default:
		return nil, fmt.Errorf("unsupported bootloader %q", bootloader)
	}
}

// CleanupBootloader cleans up the alternate bootloader when booting off via BIOS or UEFI.
func CleanupBootloader(disk string, sdboot bool) error {
	dev, err := block.NewFromPath(disk, block.OpenForWrite())
	if err != nil {
		return err
	}

	defer dev.Close() //nolint:errcheck

	if err := dev.Lock(true); err != nil {
		return fmt.Errorf("failed to lock device: %w", err)
	}

	defer dev.Unlock() //nolint:errcheck

	gptDev, err := gpt.DeviceFromBlockDevice(dev)
	if err != nil {
		return fmt.Errorf("failed to get GPT device: %w", err)
	}

	gptTable, err := gpt.Read(gptDev)
	if err != nil {
		return fmt.Errorf("failed to read GPT: %w", err)
	}

	if sdboot {
		// we wipe upto 446 bytes where the protective MBR is located
		if _, err := dev.WipeRange(0, 446); err != nil {
			return fmt.Errorf("failed to wipe MBR: %w", err)
		}

		if err := deletePartitions(gptTable, constants.BIOSGrubPartitionLabel, constants.BootPartitionLabel); err != nil {
			return err
		}
	} else {
		// means we are using GRUB
		if err := deletePartitions(gptTable, constants.EFIPartitionLabel); err != nil {
			return err
		}
	}

	if err := gptTable.Write(); err != nil {
		return fmt.Errorf("failed to write GPT: %w", err)
	}

	return nil
}

func deletePartitions(gptTable *gpt.Table, labels ...string) error {
	for i, part := range gptTable.Partitions() {
		if part == nil {
			continue
		}

		for _, label := range labels {
			if part.Name == label {
				if err := gptTable.DeletePartition(i); err != nil {
					return fmt.Errorf("failed to delete partition %s %d: %w", part.Name, i, err)
				}
			}
		}
	}

	return nil
}
