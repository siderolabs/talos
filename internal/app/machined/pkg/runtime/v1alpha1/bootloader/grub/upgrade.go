// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"path/filepath"

	"github.com/siderolabs/go-blockdevice/v2/blkid"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Upgrade copies new boot assets and updates grub configuration on an existing installation.
func (c *Config) Upgrade(opts options.InstallOptions) (*options.InstallResult, error) {
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
			if err := c.flip(); err != nil {
				return err
			}

			if err := c.generateAssets(opts, constants.EFIMountPoint); err != nil {
				return err
			}

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
