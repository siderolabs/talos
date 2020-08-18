// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import "github.com/talos-systems/talos/pkg/machinery/constants"

const (
	// BootA is a bootloader label.
	BootA = "boot-a"

	// BootB is a bootloader label.
	BootB = "boot-b"

	// GrubConfig is the path to the Syslinux config.
	GrubConfig = constants.BootMountPoint + "/grub/grub.cfg"
)
