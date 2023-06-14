// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/bootloader"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/version"
)

// Config represents a grub configuration file (grub.cfg).
type Config struct {
	Default  bootloader.BootLabel
	Fallback bootloader.BootLabel
	Entries  map[bootloader.BootLabel]MenuEntry
}

// MenuEntry represents a grub menu entry in the grub config file.
type MenuEntry struct {
	Name    string
	Linux   string
	Cmdline string
	Initrd  string
}

func (e bootloaderNotInstalledError) Error() string {
	return bootloaderNotInstalled
}

// NewConfig creates a new grub configuration (nothing is written to disk).
func NewConfig() *Config {
	return &Config{
		Default: bootloader.BootA,
		Entries: map[bootloader.BootLabel]MenuEntry{},
	}
}

// UEFIBoot returns true if bootloader is UEFI-only.
func (c *Config) UEFIBoot() bool {
	// grub supports BIOS boot, so false here.
	return false
}

// Put puts a new menu entry to the grub config (nothing is written to disk).
func (c *Config) Put(entry bootloader.BootLabel, cmdline string) error {
	c.Entries[entry] = buildMenuEntry(entry, cmdline)

	return nil
}

func (c *Config) validate() error {
	if _, ok := c.Entries[c.Default]; !ok {
		return fmt.Errorf("invalid default entry: %s", c.Default)
	}

	if c.Fallback != "" {
		if _, ok := c.Entries[c.Fallback]; !ok {
			return fmt.Errorf("invalid fallback entry: %s", c.Fallback)
		}
	}

	if c.Default == c.Fallback {
		return fmt.Errorf("default and fallback entries must not be the same")
	}

	return nil
}

func buildMenuEntry(entry bootloader.BootLabel, cmdline string) MenuEntry {
	return MenuEntry{
		Name:    fmt.Sprintf("%s - %s", entry, version.Short()),
		Linux:   filepath.Join("/", string(entry), constants.KernelAsset),
		Cmdline: cmdline,
		Initrd:  filepath.Join("/", string(entry), constants.InitramfsAsset),
	}
}
