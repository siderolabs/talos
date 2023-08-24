// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package mount provides bootloader mount operations.
package mount

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"

	"github.com/siderolabs/talos/internal/pkg/mount"
)

// PartitionOp mounts a partition with the specified label, executes the operation func, and unmounts the partition.
//
//nolint:gocyclo
func PartitionOp(ctx context.Context, disk string, partitionLabel string, opFunc func() error) error {
	var probedBlockDevice *blockdevice.BlockDevice

	switch {
	case disk != "":
		dev, err := blockdevice.Open(disk, blockdevice.WithMode(blockdevice.ReadonlyMode))
		if err != nil {
			return err
		}

		defer dev.Close() //nolint:errcheck

		_, err = dev.GetPartition(partitionLabel)
		if err != nil {
			if errors.Is(err, blockdevice.ErrMissingPartitionTable) || os.IsNotExist(err) {
				return nil
			}

			return err
		}

		probedBlockDevice = dev
	case disk == "":
		// attempt to probe partition with partitionLabel on any disk
		dev, err := probe.GetDevWithPartitionName(partitionLabel)
		if os.IsNotExist(err) {
			// no EFI partition, nothing to do
			return nil
		}

		if err != nil {
			return err
		}

		defer dev.Close() //nolint:errcheck

		probedBlockDevice = dev.BlockDevice
	}

	mp, err := mount.SystemMountPointForLabel(ctx, probedBlockDevice, partitionLabel, mount.WithFlags(mount.ReadOnly))
	if err != nil {
		return err
	}

	// no mountpoint defined for this partition, should not happen
	if mp == nil {
		return fmt.Errorf("no mountpoint defined for %s", partitionLabel)
	}

	alreadyMounted, err := mp.IsMounted()
	if err != nil {
		return err
	}

	if !alreadyMounted {
		if err = mp.Mount(); err != nil {
			return err
		}

		defer mp.Unmount() //nolint:errcheck
	}

	return opFunc()
}

// GetBlockDeviceName returns the block device name for the specified boot disk and partition label.
func GetBlockDeviceName(bootDisk, partitionLabel string) (string, error) {
	dev, err := blockdevice.Open(bootDisk, blockdevice.WithMode(blockdevice.ReadonlyMode))
	if err != nil {
		return "", err
	}

	//nolint:errcheck
	defer dev.Close()

	// verify that BootDisk has partition with the specified label
	_, err = dev.GetPartition(partitionLabel)
	if err != nil {
		return "", err
	}

	blk := dev.Device().Name()

	return blk, nil
}
