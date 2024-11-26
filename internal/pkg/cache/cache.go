// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cache provides methods to install an image cache
package cache

import (
	"fmt"
	"os"

	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-copy/copy"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// InstallOptions contains the options for installing the cache.
type InstallOptions struct {
	// The disk where cache partition is present.
	CacheDisk string
	// Source of the cache from where it will be copied.
	CachePath string
	// Optional: blkid probe result.
	BlkidInfo *blkid.Info
}

// Install installs the cache to the given disk.
func (i *InstallOptions) Install() error {
	tempMountDir, err := os.MkdirTemp("", "talos-image-cache-install")
	if err != nil {
		return fmt.Errorf("creating temporary directory for talos-image-cache-install: %w", err)
	}

	defer os.RemoveAll(tempMountDir) //nolint:errcheck

	return mount.PartitionOp(
		i.CacheDisk,
		[]mount.Spec{
			{
				PartitionLabel: constants.ImageCachePartitionLabel,
				FilesystemType: partition.FileSystemTypeExt4,
				MountTarget:    tempMountDir,
			},
		},
		func() error {
			return copy.Dir(i.CachePath, tempMountDir)
		},
		[]blkid.ProbeOption{
			// installation happens with locked blockdevice
			blkid.WithSkipLocking(true),
		},
		nil,
		nil,
		i.BlkidInfo,
	)
}
