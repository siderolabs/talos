/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manifest

import (
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// VerifyDataDevice verifies the supplied data device options.
func VerifyDataDevice(install machine.Install) (err error) {
	if install.Disk() == "" {
		return errors.New("missing disk")
	}

	if install.Force() {
		return nil
	}

	if err = VerifyDiskAvailability(install.Disk(), constants.EphemeralPartitionLabel); err != nil {
		return errors.Wrap(err, "failed to verify disk availability")
	}

	return nil
}

// VerifyBootDevice verifies the supplied boot device options.
func VerifyBootDevice(install machine.Install) (err error) {
	if !install.WithBootloader() {
		return nil
	}

	if install.Force() {
		return nil
	}

	if err = VerifyDiskAvailability(install.Disk(), constants.BootPartitionLabel); err != nil {
		return errors.Wrap(err, "failed to verify disk availability")
	}

	return nil
}

// VerifyDiskAvailability verifies that no filesystems currently exist with
// the labels used by the OS.
func VerifyDiskAvailability(devpath, label string) (err error) {
	var dev *probe.ProbedBlockDevice
	if dev, err = probe.DevForFileSystemLabel(devpath, label); err != nil {
		// We return here because we only care if we can discover the
		// device successfully and confirm that the disk is not in use.
		// TODO(andrewrynhard): We should return a custom error type here
		// that we can use to confirm the device was not found.
		return nil
	}
	if dev.SuperBlock != nil {
		return errors.Errorf("target install device %s is not empty, found existing %s file system", label, dev.SuperBlock.Type())
	}

	return nil
}
