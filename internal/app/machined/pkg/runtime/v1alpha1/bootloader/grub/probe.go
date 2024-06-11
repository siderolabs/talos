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

// Probe probes a block device for GRUB bootloader.
func Probe(disk string, options options.ProbeOptions) (*Config, error) {
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

			return nil
		},
		options.BlockProbeOptions,
		[]mountv2.NewPointOption{
			mountv2.WithReadonly(),
		},
		[]mountv2.MountOption{
			mountv2.WithSkipIfMounted(),
		},
	); err != nil {
		if xerrors.TagIs[mount.NotFoundTag](err) {
			// if partitions are not found, it means GRUB is not installed
			return nil, nil
		}

		return nil, err
	}

	return grubConf, nil
}
