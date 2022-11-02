// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/version"
)

// Config represents a grub configuration file (grub.cfg).
type Config struct {
	Default  BootLabel
	Fallback BootLabel
	Entries  map[BootLabel]MenuEntry
}

// MenuEntry represents a grub menu entry in the grub config file.
type MenuEntry struct {
	Name    string
	Linux   string
	Cmdline string
	Initrd  string
}

// NewConfig creates a new grub configuration (nothing is written to disk).
func NewConfig(cmdline string) *Config {
	return &Config{
		Default: BootA,
		Entries: map[BootLabel]MenuEntry{
			BootA: buildMenuEntry(BootA, cmdline),
		},
	}
}

// Put puts a new menu entry to the grub config (nothing is written to disk).
func (c *Config) Put(entry BootLabel, cmdline string) error {
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

func buildMenuEntry(entry BootLabel, cmdline string) MenuEntry {
	return MenuEntry{
		Name:    fmt.Sprintf("%s - %s", entry, version.Short()),
		Linux:   filepath.Join("/", string(entry), constants.KernelAsset),
		Cmdline: cmdline,
		Initrd:  filepath.Join("/", string(entry), constants.InitramfsAsset),
	}
}
