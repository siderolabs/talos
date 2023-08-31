// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	amd64 = "amd64"
	arm64 = "arm64"
)

// Install validates the grub configuration and writes it to the disk.
//
//nolint:gocyclo
func (c *Config) Install(options options.InstallOptions) error {
	if err := c.flip(); err != nil {
		return err
	}

	options.BootAssets.FillDefaults(options.Arch)

	if err := utils.CopyFiles(
		options.Printf,
		utils.SourceDestination(
			options.BootAssets.KernelPath,
			filepath.Join(options.MountPrefix, constants.BootMountPoint, string(c.Default), constants.KernelAsset),
		),
		utils.SourceDestination(
			options.BootAssets.InitramfsPath,
			filepath.Join(options.MountPrefix, constants.BootMountPoint, string(c.Default), constants.InitramfsAsset),
		),
	); err != nil {
		return err
	}

	if err := c.Put(c.Default, options.Cmdline, options.Version); err != nil {
		return err
	}

	if err := c.Write(filepath.Join(options.MountPrefix, ConfigPath), options.Printf); err != nil {
		return err
	}

	blk, err := getBlockDeviceName(options.BootDisk)
	if err != nil {
		return err
	}

	var platforms []string

	switch options.Arch {
	case amd64:
		platforms = []string{"x86_64-efi", "i386-pc"}
	case arm64:
		platforms = []string{"arm64-efi"}
	}

	if runtime.GOARCH == amd64 && options.Arch == amd64 && !options.ImageMode {
		// let grub choose the platform automatically if not building an image
		platforms = []string{""}
	}

	for _, platform := range platforms {
		args := []string{
			"--boot-directory=" + filepath.Join(options.MountPrefix, constants.BootMountPoint),
			"--efi-directory=" + filepath.Join(options.MountPrefix, constants.EFIMountPoint),
			"--removable",
		}

		if options.ImageMode {
			args = append(args, "--no-nvram")
		}

		if platform != "" {
			args = append(args, "--target="+platform)
		}

		args = append(args, blk)

		options.Printf("executing: grub-install %s", strings.Join(args, " "))

		if _, err := cmd.Run("grub-install", args...); err != nil {
			return fmt.Errorf("failed to install grub: %w", err)
		}
	}

	return nil
}

func getBlockDeviceName(bootDisk string) (string, error) {
	dev, err := blockdevice.Open(bootDisk, blockdevice.WithMode(blockdevice.ReadonlyMode))
	if err != nil {
		return "", err
	}

	//nolint:errcheck
	defer dev.Close()

	// verify that BootDisk has boot partition
	_, err = dev.GetPartition(constants.BootPartitionLabel)
	if err != nil {
		return "", err
	}

	blk := dev.Device().Name()

	return blk, nil
}
