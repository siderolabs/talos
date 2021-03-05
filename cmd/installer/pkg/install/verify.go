// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"errors"
	"fmt"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// VerifyEphemeralPartition verifies the supplied data device options.
func VerifyEphemeralPartition(opts *Options) (err error) {
	if opts.Disk == "" {
		return errors.New("missing disk")
	}

	if opts.Force {
		return nil
	}

	if err = VerifyDiskAvailability(opts.Disk, constants.EphemeralPartitionLabel); err != nil {
		return fmt.Errorf("failed to verify disk availability: %w", err)
	}

	return nil
}

// VerifyBootPartition verifies the supplied boot device options.
func VerifyBootPartition(opts *Options) (err error) {
	if opts.Bootloader {
		return nil
	}

	if opts.Force {
		return nil
	}

	if err = VerifyDiskAvailability(opts.Disk, constants.BootPartitionLabel); err != nil {
		return fmt.Errorf("failed to verify disk availability: %w", err)
	}

	return nil
}

// VerifyDiskAvailability verifies that no filesystems currently exist with
// the labels used by the OS.
func VerifyDiskAvailability(devpath, label string) (err error) {
	var dev *blockdevice.BlockDevice

	if dev, err = blockdevice.Open(devpath); err != nil {
		// We return here because we only care if we can discover the
		// device successfully and confirm that the disk is not in use.
		// TODO(andrewrynhard): We should return a custom error type here
		// that we can use to confirm the device was not found.
		return nil
	}

	//nolint:errcheck
	defer dev.Close()

	part, err := dev.GetPartition(label)
	if err != nil {
		return err
	}

	fsType, err := part.Filesystem()
	if err != nil {
		return err
	}

	if fsType != filesystem.Unknown {
		return fmt.Errorf("target install device %s is not empty, found existing %s file system", label, fsType)
	}

	return nil
}
