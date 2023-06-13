// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sdboot provides the interface to the Systemd-Boot bootloader: config management, installation, etc.
package sdboot

import (
	"os"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/bootloader"
	"github.com/siderolabs/talos/internal/pkg/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

type Config struct {
	Next     bootloader.BootLabel
	Fallback bootloader.BootLabel
}

func isUEFIBoot() bool {
	// https://renenyffenegger.ch/notes/Linux/fhs/sys/firmware/efi/index
	_, err := os.Stat("/sys/firmware/efi")

	return err == nil
}

func isUsingSDBoot() bool {
	// https://www.freedesktop.org/software/systemd/man/systemd-stub.html#EFI%20Variables
	// https://www.freedesktop.org/software/systemd/man/systemd-stub.html#StubInfo
	_, err := os.Stat("/sys/firmware/efi/efivars/StubInfo-4a67b082-0a4c-41cf-b6c7-440b29bb8c4f")

	return err == nil
}

func Probe() (*Config, error) {
	// if not UEFI boot, nothing to do
	if !isUEFIBoot() {
		return nil, nil
	}

	// mount the efivars filesystem to see if sd-boot is being used
	mp := mount.NewMountPoint("", constants.EFIVarsMountPoint, "efivarfs", 0, "")

	alreadyMounted, err := mp.IsMounted()
	if err != nil {
		return nil, err
	}

	if !alreadyMounted {
		if err = mp.Mount(); err != nil {
			return nil, err
		}

		defer mp.Unmount() //nolint:errcheck
	}

	if !isUsingSDBoot() {
		return nil, nil
	}

	// here we need to read the EFI vars to see if we have any defaults
	// and populate config accordingly
	// https://www.freedesktop.org/software/systemd/man/systemd-boot.html#LoaderEntryDefault
	// this should be set on install/upgrades

	return nil, nil
}

func (c *Config) Install(bootDisk, arch, cmdline string) error {
	// install should set the EFI vars only
	return nil
}

func (c *Config) Installed() bool {
	// this would only work if we populate Config in Probe with values from EFI vars
	return c != nil
}

// String returns the bootloader name.
func (c *Config) String() string {
	return "sdboot"
}

func (c *Config) Flip() error {
	return nil
}

func (c *Config) Revert() error {
	return nil
}

func (c *Config) NextLabel() string {
	// If the bootloader is not installed, return the default label.
	if c == nil {
		return string(bootloader.BootA)
	}

	return string(c.Next)
}

func (c *Config) PreviousLabel() string {
	// If the bootloader is not installed, empty.
	if c == nil {
		return ""
	}

	return string(c.Fallback)
}
