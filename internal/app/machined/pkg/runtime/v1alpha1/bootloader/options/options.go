// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package options provides bootloader options.
package options

import (
	"fmt"

	"github.com/siderolabs/go-blockdevice/v2/blkid"

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

	// Mount prefix for /boot-like partitions.
	MountPrefix string

	// Boot assets to install.
	BootAssets BootAssets

	// ExtraInstallStep is a function to run after the bootloader is installed.
	ExtraInstallStep func() error

	// Printf-like function to use.
	Printf func(format string, v ...any)

	// Optional: blkid probe result.
	BlkidInfo *blkid.Info
}

// InstallResult is the result of the installation.
type InstallResult struct {
	// Previous label (if upgrading).
	PreviousLabel string
}

// BootAssets describes the assets to be installed by the bootloader.
type BootAssets struct {
	KernelPath    string
	InitramfsPath string

	UKIPath    string
	SDBootPath string

	DTBPath         string
	UBootPath       string
	RPiFirmwarePath string
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

	if arch == "arm64" {
		if assets.DTBPath == "" {
			assets.DTBPath = fmt.Sprintf(constants.DTBAssetPath, arch)
		}

		if assets.UBootPath == "" {
			assets.UBootPath = fmt.Sprintf(constants.UBootAssetPath, arch)
		}

		if assets.RPiFirmwarePath == "" {
			assets.RPiFirmwarePath = fmt.Sprintf(constants.RPiFirmwareAssetPath, arch)
		}
	}
}

// ProbeOptions configures bootloader probing.
type ProbeOptions struct {
	BlockProbeOptions []blkid.ProbeOption
}
