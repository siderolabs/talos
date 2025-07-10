// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"github.com/siderolabs/gen/xerrors"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ProbeWithCallback probes the GRUB bootloader, and calls the callback function with the Config.
func ProbeWithCallback(disk string, options options.ProbeOptions, callback func(*Config) error) (*Config, error) {
	var grubConf *Config

	if err := mount.PartitionOp(
		disk,
		[]mount.Spec{
			{
				PartitionLabel: constants.BootPartitionLabel,
				FilesystemType: partition.FilesystemTypeXFS,
				MountTarget:    constants.BootMountPoint,
			},
		},
		func() error {
			var err error

			grubConf, err = Read(ConfigPath)
			if err != nil {
				return err
			}

			if grubConf != nil && callback != nil {
				return callback(grubConf)
			}

			if grubConf == nil {
				options.Log("GRUB: config not found")
			}

			return nil
		},
		options.BlockProbeOptions,
		[]mountv2.NewPointOption{
			mountv2.WithReadonly(),
		},
		[]mountv2.OperationOption{
			mountv2.WithSkipIfMounted(),
		},
		nil,
	); err != nil {
		if xerrors.TagIs[mount.NotFoundTag](err) {
			// if partitions are not found, it means GRUB is not installed
			options.Log("GRUB: BOOT partition not found, skipping probing")

			return nil, nil
		}

		return nil, err
	}

	return grubConf, nil
}

// Probe probes a block device for GRUB bootloader.
func Probe(disk string, options options.ProbeOptions) (*Config, error) {
	return ProbeWithCallback(disk, options, nil)
}
