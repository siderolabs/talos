// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package syslinux

import "github.com/talos-systems/talos/pkg/universe"

const (
	// BootA is a syslinux label.
	BootA = "boot-a"

	// BootB is a syslinux label.
	BootB = "boot-b"

	// SyslinuxLdlinux is the path to ldlinux.sys.
	SyslinuxLdlinux = universe.BootMountPoint + "/syslinux/ldlinux.sys"

	// SyslinuxConfig is the path to the Syslinux config.
	SyslinuxConfig = universe.BootMountPoint + "/syslinux/syslinux.cfg"

	gptmbrbin   = "/usr/lib/syslinux/gptmbr.bin"
	syslinuxefi = "/usr/lib/syslinux/syslinux.efi"
	ldlinuxe64  = "/usr/lib/syslinux/ldlinux.e64"
)
