// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// BootLabel represents a boot label, e.g. A or B.
type BootLabel string

const (
	// ConfigPath is the path to the grub config.
	ConfigPath = constants.BootMountPoint + "/grub/grub.cfg"
	// BootA is a bootloader label.
	BootA BootLabel = "A"
	// BootB is a bootloader label.
	BootB BootLabel = "B"
	// BootReset is a bootloader label.
	BootReset BootLabel = "Reset"
)

const (
	bootloaderNotInstalled = "bootloader not installed"
)

type bootloaderNotInstalledError struct{}
