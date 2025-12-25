// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// Config represents a grub configuration file (grub.cfg).
type Config struct {
	Default        BootLabel
	Fallback       BootLabel
	Entries        map[BootLabel]MenuEntry
	AddResetOption bool
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
		Default:        BootA,
		Entries:        map[BootLabel]MenuEntry{},
		AddResetOption: true,
	}
}

// KexecLoad does a kexec using the bootloader config.
func (c *Config) KexecLoad(r runtime.Runtime, disk string) error {
	_, err := ProbeWithCallback(disk, options.ProbeOptions{}, func(grubConf *Config) error {
		defaultEntry, ok := grubConf.Entries[grubConf.Default]

		if !ok {
			return nil
		}

		kernelPath := filepath.Join(constants.BootMountPoint, defaultEntry.Linux)
		initrdPath := filepath.Join(constants.BootMountPoint, defaultEntry.Initrd)

		kernel, err := os.Open(kernelPath)
		if err != nil {
			return err
		}

		defer kernel.Close() //nolint:errcheck

		initrd, err := os.Open(initrdPath)
		if err != nil {
			return err
		}

		defer initrd.Close() //nolint:errcheck

		cmdline := strings.TrimSpace(defaultEntry.Cmdline)

		if err = kexec.Load(r, kernel, int(initrd.Fd()), cmdline); err != nil {
			return err
		}

		log.Printf("prepared kexec environment kernel=%q initrd=%q cmdline=%q", kernelPath, initrdPath, cmdline)

		return nil
	})

	return err
}

// GenerateAssets generates the bootloader assets and returns partition options to create the bootloader partitions.
func (c *Config) GenerateAssets(efiAssetsPath string, opts options.InstallOptions) ([]partition.Options, error) {
	if err := c.generateAssets(opts, efiAssetsPath); err != nil {
		return nil, err
	}

	quirk := quirks.New(opts.Version)

	efiFormatOptions := []partition.FormatOption{
		partition.WithLabel(constants.EFIPartitionLabel),
	}

	if opts.ImageMode {
		// in bios install mode grub generated assets only contains the grub config file and kernel and initramfs
		// so we don't need to set the source directory for the EFI partition
		efiFormatOptions = append(
			efiFormatOptions,
			partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, efiAssetsPath)),
		)
	}

	partitionOptions := []partition.Options{
		partition.NewPartitionOptions(
			false,
			quirk,
			efiFormatOptions...,
		),
		partition.NewPartitionOptions(false, quirk, partition.WithLabel(constants.BIOSGrubPartitionLabel)),
		partition.NewPartitionOptions(
			false,
			quirk,
			partition.WithLabel(constants.BootPartitionLabel),
			partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, constants.BootMountPoint)),
		),
	}

	if opts.ImageMode {
		partitionOptions = xslices.Map(partitionOptions, func(o partition.Options) partition.Options {
			o.Reproducible = true

			return o
		})
	}

	if opts.ExtraInstallStep != nil {
		if err := opts.ExtraInstallStep(); err != nil {
			return nil, err
		}
	}

	return partitionOptions, nil
}

// Put puts a new menu entry to the grub config (nothing is written to disk).
func (c *Config) Put(entry BootLabel, cmdline, version string) error {
	c.Entries[entry] = buildMenuEntry(entry, cmdline, version)

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
		return errors.New("default and fallback entries must not be the same")
	}

	return nil
}

func buildMenuEntry(entry BootLabel, cmdline, versionTag string) MenuEntry {
	return MenuEntry{
		Name:    fmt.Sprintf("%s - %s %s", entry, version.Name, versionTag),
		Linux:   filepath.Join("/", string(entry), constants.KernelAsset),
		Cmdline: cmdline,
		Initrd:  filepath.Join("/", string(entry), constants.InitramfsAsset),
	}
}
