/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package installer

import (
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// VerifyDataDevice verifies the supplied data device options.
func VerifyDataDevice(data *userdata.UserData) (err error) {
	// Set data device to root device if not specified
	if data.Install.Ephemeral == nil {
		data.Install.Ephemeral = &userdata.InstallDevice{}
	}

	if data.Install.Ephemeral.Device == "" {
		return errors.New("an ephemeral device is required")
	}

	if !data.Install.Force {
		if err = VerifyDiskAvailability(constants.EphemeralPartitionLabel); err != nil {
			return errors.Wrap(err, "failed to verify disk availability")
		}
	}

	return nil
}

// VerifyBootDevice verifies the supplied boot device options.
func VerifyBootDevice(data *userdata.UserData) (err error) {
	if data.Install.Boot == nil {
		return nil
	}

	if data.Install.Boot.Device == "" {
		// We can safely assume data device is defined at this point
		// because VerifyDataDevice should have been called first in
		// in the chain
		data.Install.Boot.Device = data.Install.Ephemeral.Device
	}

	if data.Install.Boot.Size == 0 {
		data.Install.Boot.Size = DefaultSizeBootDevice
	}

	if data.Install.Boot.Kernel == "" {
		data.Install.Boot.Kernel = DefaultKernelURL
	}

	if data.Install.Boot.Initramfs == "" {
		data.Install.Boot.Initramfs = DefaultInitramfsURL
	}

	if !data.Install.Force {
		if err = VerifyDiskAvailability(constants.BootPartitionLabel); err != nil {
			return errors.Wrap(err, "failed to verify disk availability")
		}
	}
	return nil
}

// VerifyDiskAvailability verifies that no filesystems currently exist with
// the labels used by the OS.
func VerifyDiskAvailability(label string) (err error) {
	var dev *probe.ProbedBlockDevice
	if dev, err = probe.GetDevWithFileSystemLabel(label); err != nil {
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
