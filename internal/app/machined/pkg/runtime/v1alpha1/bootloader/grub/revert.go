// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-blockdevice/blockdevice/probe"

	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Revert reverts the bootloader to the previous version.
// nolint:gocyclo
func (c *Config) Revert(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("cannot revert bootloader: %w", bootloaderNotInstalledError{})
	}

	if err := c.flip(); err != nil {
		return err
	}

	// attempt to probe BOOT partition directly
	dev, err := probe.GetDevWithPartitionName(constants.BootPartitionLabel)
	if os.IsNotExist(err) {
		// no BOOT partition, nothing to revert
		return nil
	}

	if err != nil {
		return err
	}

	defer dev.Close() //nolint:errcheck

	mp, err := mount.SystemMountPointForLabel(ctx, dev.BlockDevice, constants.BootPartitionLabel)
	if err != nil {
		return err
	}

	// if no BOOT partition nothing to revert
	if mp == nil {
		return nil
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

	if _, err = os.Stat(filepath.Join(constants.BootMountPoint, string(c.Default))); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot rollback to %q, label does not exist", "")
	}

	if err := c.Write(ConfigPath, log.Printf); err != nil {
		return fmt.Errorf("failed to revert bootloader: %v", err)
	}

	return nil
}
