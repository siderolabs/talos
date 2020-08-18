// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import "github.com/talos-systems/talos/pkg/machinery/constants"

const (
	// BootA is a bootloader label.
	BootA = "A"

	// BootB is a bootloader label.
	BootB = "B"

	// GrubConfig is the path to the grub config.
	GrubConfig = constants.BootMountPoint + "/grub/grub.cfg"

	// GrubDeviceMap is the path to the grub device map.
	GrubDeviceMap = constants.BootMountPoint + "/grub/device.map"
)
