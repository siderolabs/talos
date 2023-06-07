// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"fmt"
	"os"

	"github.com/siderolabs/go-blockdevice/blockdevice/probe"

	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Probe probes a block device for GRUB bootloader.
func Probe() (*Config, error) {
	// attempt to probe BOOT partition directly
	dev, err := probe.GetDevWithPartitionName(constants.BootPartitionLabel)
	if os.IsNotExist(err) {
		// no BOOT partition, nothing to do
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	defer dev.Close() //nolint:errcheck

	mp, err := mount.SystemMountPointForLabel(dev.BlockDevice, constants.BootPartitionLabel)
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
