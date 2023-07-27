// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package options provides bootloader options.
package options

import (
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// InstallOptions configures bootloader installation.
type InstallOptions struct {
	// The disk to install to.
	BootDisk string
	// Target architecture.
	Arch string
	// Kernel command line (grub only).
	Cmdline string
	// Talos version.
	Version string

	// Are we running in image mode?
	ImageMode bool

	// Boot assets to install.
	BootAssets BootAssets
}

// BootAssets describes the assets to be installed by the booloader.
type BootAssets struct {
	KernelPath    string
	InitramfsPath string

	UKIPath    string
	SDBootPath string
}

// FillDefaults fills in default paths to be used when in the context of the installer.
func (assets *BootAssets) FillDefaults(arch string) {
	if assets.KernelPath == "" {
		assets.KernelPath = fmt.Sprintf(constants.KernelAssetPath, arch)
	}

	if assets.InitramfsPath == "" {
		assets.InitramfsPath = fmt.Sprintf(constants.InitramfsAssetPath, arch)
	}

	if assets.UKIPath == "" {
		assets.UKIPath = fmt.Sprintf(constants.UKIAssetPath, arch)
	}

	if assets.SDBootPath == "" {
		assets.SDBootPath = fmt.Sprintf(constants.SDBootAssetPath, arch)
	}
}
