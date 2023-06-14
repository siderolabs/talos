// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"errors"
	"fmt"
	"os"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"

	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Probe probes a block device for GRUB bootloader.
//
// If the 'disk' is passed, search happens on that disk only, otherwise searches all partitions.
//
//nolint:gocyclo
func Probe(disk string) (*Config, error) {
	var probedBlockDevice *blockdevice.BlockDevice

	switch {
	case disk != "":
		dev, err := blockdevice.Open(disk, blockdevice.WithMode(blockdevice.ReadonlyMode))
		if err != nil {
			return nil, err
		}

		defer dev.Close() //nolint:errcheck

		_, err = dev.GetPartition(constants.BootPartitionLabel)
		if err != nil {
			if errors.Is(err, blockdevice.ErrMissingPartitionTable) || os.IsNotExist(err) {
				return nil, nil
			}

			return nil, err
		}

		probedBlockDevice = dev
	case disk == "":
		// attempt to probe BOOT partition on any disk
		dev, err := probe.GetDevWithPartitionName(constants.BootPartitionLabel)
		if os.IsNotExist(err) {
			// no BOOT partition, nothing to do
			return nil, nil
		}

		if err != nil {
			return nil, err
		}

		defer dev.Close() //nolint:errcheck

		probedBlockDevice = dev.BlockDevice
	}

	mp, err := mount.SystemMountPointForLabel(probedBlockDevice, constants.BootPartitionLabel, mount.WithFlags(mount.ReadOnly))
	if err != nil {
		return nil, err
	}

	// no mountpoint defined for this partition, should not happen
	if mp == nil {
		return nil, fmt.Errorf("no mountpoint defined for %s", constants.BootPartitionLabel)
	}

	alreadyMounted, err := mp.IsMounted()
	if err != nil {
		return nil, err
	}

	if !alreadyMounted {
		if err = mp.Mount(); err != nil {
			return nil, err
		}

		defer mp.Unmount() //nolint:errcheck
	}

	grubConf, err := Read(ConfigPath)
	if err != nil {
		return nil, err
	}

	return grubConf, nil
}
