// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// BootPartitionVariable is the GRUB variable holding the partition UUID of the partition GRUB was loaded from (BOOT).
//
// It is set by `probe --part-uuid $root` in the generated config, and passed to the kernel via constants.KernelParamBootPartitionUUID.
const BootPartitionVariable = "talos_bootpart"

// bootPartitionCmdlineArg is the kernel argument appended to the `linux` command, expanded by GRUB at boot.
const bootPartitionCmdlineArg = constants.KernelParamBootPartitionUUID + "=$" + BootPartitionVariable

// Config represents a grub configuration file (grub.cfg).
type Config struct {
	Default        BootLabel
	Fallback       BootLabel
	Entries        map[BootLabel]MenuEntry
	AddResetOption bool
	// AppendBootPartitionUUID makes GRUB probe the partition it was loaded from (BOOT) and pass its UUID
	// to the kernel via the `talos.boot.partuuid` argument.
	AppendBootPartitionUUID bool
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
		Default:                 BootA,
		Entries:                 map[BootLabel]MenuEntry{},
		AddResetOption:          true,
		AppendBootPartitionUUID: true,
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

		// GRUB is skipped on kexec, so the boot partition UUID it would have probed is round-tripped
		// from the current boot (if it is known)
		if c.AppendBootPartitionUUID {
			cmdline = kexec.AppendBootPartitionUUID(cmdline, bootPartitionUUID(r))
		}

		if err = kexec.Load(r, kernel, int(initrd.Fd()), cmdline); err != nil {
			return err
		}

		log.Printf("prepared kexec environment kernel=%q initrd=%q cmdline=%q", kernelPath, initrdPath, cmdline)

		return nil
	})

	return err
}

// PrepareBootPartitions prepares the set of partitions to create for the bootloader.
//
// In image mode, this also pre-populates the assets to be written to the bootloader partitions
// when formatting the filesystem.
// In install mode, this only returns a list of partitions to create, and the assets are written to the partitions during Install.
func (c *Config) PrepareBootPartitions(opts options.InstallOptions) ([]partition.Options, error) {
	quirk := quirks.New(opts.Version)

	efiFormatOptions := []partition.FormatOption{
		partition.WithLabel(constants.EFIPartitionLabel),
	}

	bootFormatOptions := []partition.FormatOption{
		partition.WithLabel(constants.BootPartitionLabel),
	}

	if opts.ImageMode {
		efiFormatOptions = append(
			efiFormatOptions,
			partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, "EFI")),
		)

		bootFormatOptions = append(
			bootFormatOptions,
			partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, constants.BootMountPoint)),
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
			bootFormatOptions...,
		),
	}

	if opts.ImageMode {
		partitionOptions = xslices.Map(partitionOptions, func(o partition.Options) partition.Options {
			o.Reproducible = true

			return o
		})

		if err := c.copyAssets(opts); err != nil {
			return nil, err
		}

		if opts.ExtraInstallStep != nil {
			if err := opts.ExtraInstallStep(); err != nil {
				return nil, err
			}
		}

		// the EFI assets (both GRUB's own, and the ones written by the overlay installer) are staged
		// under constants.EFIMountPoint to match the layout of the install mode, so move them out of
		// the BOOT partition source directory into the EFI partition source directory
		if err := os.Rename(filepath.Join(opts.MountPrefix, constants.EFIMountPoint), filepath.Join(opts.MountPrefix, "EFI")); err != nil {
			return nil, fmt.Errorf("failed to move EFI directory: %w", err)
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

// bootPartitionUUID returns the boot partition UUID detected on the current boot, or an empty string if it's not known.
func bootPartitionUUID(r runtime.Runtime) string {
	status, err := safe.StateGetByID[*runtimeres.BootPartitionStatus](context.Background(), r.State().V1Alpha2().Resources(), runtimeres.BootPartitionStatusID)
	if err != nil {
		if !state.IsNotFoundError(err) {
			log.Printf("error getting the boot partition status: %s", err)
		}

		return ""
	}

	return status.TypedSpec().PartitionUUID
}
