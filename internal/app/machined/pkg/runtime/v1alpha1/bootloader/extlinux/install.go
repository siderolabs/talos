// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extlinux

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/go-blockdevice/v2/blkid"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Install implements the Bootloader interface.
// It installs the extlinux bootloader configuration and boot assets.
func (c *Config) Install(opts options.InstallOptions) (*options.InstallResult, error) {
	var installResult *options.InstallResult

	// Mount EFI partition
	mountSpecs := []mount.Spec{
		{
			PartitionLabel: constants.EFIPartitionLabel,
			FilesystemType: partition.FilesystemTypeVFAT,
			MountTarget:    filepath.Join(opts.MountPrefix, constants.EFIMountPoint),
		},
	}

	err := mount.PartitionOp(
		opts.BootDisk,
		mountSpecs,
		func() error {
			var installErr error

			installResult, installErr = c.install(opts)

			return installErr
		},
		[]blkid.ProbeOption{
			blkid.WithSkipLocking(true),
		},
		nil,
		nil,
		opts.BlkidInfo,
	)

	return installResult, err
}

func (c *Config) install(opts options.InstallOptions) (*options.InstallResult, error) {
	efiMountPoint := filepath.Join(opts.MountPrefix, constants.EFIMountPoint)

	// Create extlinux directory
	extlinuxDir := filepath.Join(efiMountPoint, "extlinux")
	if err := os.MkdirAll(extlinuxDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create extlinux directory: %w", err)
	}

	// Copy kernel to root of EFI partition
	kernelDst := filepath.Join(efiMountPoint, constants.KernelAsset)

	// Copy initramfs to root of EFI partition
	initramfsDst := filepath.Join(efiMountPoint, constants.InitramfsAsset)

	// Check if we have kernel and initramfs paths
	if _, err := os.Stat(opts.BootAssets.KernelPath); err == nil {
		if err := utils.CopyFiles(
			opts.Printf,
			utils.SourceDestination(
				opts.BootAssets.KernelPath,
				kernelDst,
			),
			utils.SourceDestination(
				opts.BootAssets.InitramfsPath,
				initramfsDst,
			),
		); err != nil {
			return nil, fmt.Errorf("failed to copy boot assets: %w", err)
		}
	} else {
		return nil, fmt.Errorf("kernel path does not exist: %w (extlinux does not support UKI)", err)
	}

	// Note: DTB files will be copied by the board-specific installation
	// which happens after bootloader installation

	// Generate extlinux.conf
	confPath := filepath.Join(extlinuxDir, "extlinux.conf")
	confContent := fmt.Sprintf(`DEFAULT talos

LABEL talos
	LINUX /%s
	INITRD /%s
	APPEND %s
	FDTDIR /dtb
`, constants.KernelAsset, constants.InitramfsAsset, opts.Cmdline)

	if err := os.WriteFile(confPath, []byte(confContent), 0o644); err != nil {
		return nil, fmt.Errorf("failed to write extlinux.conf: %w", err)
	}

	opts.Printf("extlinux bootloader installed successfully")

	// Return install result
	return &options.InstallResult{
		PreviousLabel: "", // extlinux doesn't support A/B boot
	}, nil
}
