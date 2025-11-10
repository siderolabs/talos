// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extlinux provides the interface to the extlinux bootloader for U-Boot.
package extlinux

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Config represents the extlinux bootloader configuration.
type Config struct {
	// No persistent state needed for extlinux
}

// Name implements the Bootloader interface.
func (c *Config) Name() string {
	return "extlinux"
}

// RequiredPartitions implements the Bootloader interface.
// Extlinux only needs the EFI partition (FAT32) where U-Boot can read the config.
func (c *Config) RequiredPartitions(quirk quirks.Quirks) []partition.Options {
	return []partition.Options{
		partition.NewPartitionOptions(constants.EFIPartitionLabel, false, quirk),
	}
}

// Revert implements the Bootloader interface.
// Extlinux doesn't support A/B boot, so nothing to revert.
func (c *Config) Revert(disk string) error {
	return nil
}

// KexecLoad implements the Bootloader interface.
func (c *Config) KexecLoad(r runtime.Runtime, disk string) error {
	return mount.PartitionOp(
		disk,
		[]mount.Spec{
			{
				PartitionLabel: constants.EFIPartitionLabel,
				MountTarget:    constants.EFIMountPoint,
			},
		},
		func() error {
			kernelPath := filepath.Join(constants.EFIMountPoint, constants.KernelAsset)
			initrdPath := filepath.Join(constants.EFIMountPoint, constants.InitramfsAsset)

			kernel, err := os.Open(kernelPath)
			if err != nil {
				return err
			}

			defer kernel.Close() //nolint:errcheck

			initrd, err := os.Open(initrdPath)
			if err != nil {
				return err
			}

			defer initrd.Close() //nolint:errcheck

			cmdline := r.State().Platform().KernelArgs(r.Config().Machine().Install().ExtraKernelArgs()).Strings()

			return r.State().V1Alpha2().Resources().Kexec().LoadKernel(
				kernel,
				initrd,
				cmdline,
			)
		},
	)
}
