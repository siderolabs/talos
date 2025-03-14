// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sdboot provides the interface to the Systemd-Boot bootloader: config management, installation, etc.
package sdboot

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ecks/uefi/efi/efivario"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/internal/pkg/uki"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// LoaderConfBytes is the content of the loader.conf file.
//
//go:embed loader.conf
var LoaderConfBytes []byte

// Config describe sd-boot state.
type Config struct {
	Default  string
	Fallback string
}

// IsUEFIBoot returns true if the system is booted using UEFI.
func IsUEFIBoot() bool {
	// https://renenyffenegger.ch/notes/Linux/fhs/sys/firmware/efi/index
	_, err := os.Stat("/sys/firmware/efi")

	return err == nil
}

// IsBootedUsingSDBoot returns true if the system is booted using sd-boot.
func IsBootedUsingSDBoot() bool {
	// https://www.freedesktop.org/software/systemd/man/systemd-stub.html#EFI%20Variables
	// https://www.freedesktop.org/software/systemd/man/systemd-stub.html#StubInfo
	_, err := os.Stat(SystemdBootStubInfoPath)

	return err == nil
}

// New creates a new sdboot bootloader config.
func New() *Config {
	return &Config{}
}

// ProbeWithCallback probes the sd-boot bootloader, and calls the callback function with the Config.
//
//nolint:gocyclo
func ProbeWithCallback(disk string, options options.ProbeOptions, callback func(*Config) error) (*Config, error) {
	// if not UEFI boot, nothing to do
	if !IsUEFIBoot() {
		return nil, nil
	}

	// here we need to read the EFI vars to see if we have any defaults
	// and populate config accordingly
	// https://www.freedesktop.org/software/systemd/man/systemd-boot.html#LoaderEntryDefault
	// this should be set on install/upgrades
	efiCtx := efivario.NewDefaultContext()

	bootedEntry, err := ReadVariable(efiCtx, LoaderEntrySelectedName)
	if err != nil {
		return nil, err
	}

	log.Printf("booted entry: %q", bootedEntry)

	config := &Config{}

	// read /boot/EFI and find if sd-boot is already being used
	// this is to make sure sd-boot from Talos is being used and not sd-boot from another distro
	if err := mount.PartitionOp(
		disk,
		[]mount.Spec{
			{
				PartitionLabel: constants.EFIPartitionLabel,
				FilesystemType: partition.FilesystemTypeVFAT,
				MountTarget:    constants.EFIMountPoint,
			},
		},
		func() error {
			// list existing boot*.efi files in boot folder
			files, err := filepath.Glob(filepath.Join(constants.EFIMountPoint, "EFI", "boot", "BOOT*.efi"))
			if err != nil {
				return err
			}

			if len(files) == 0 {
				return fmt.Errorf("no boot*.efi files found in %q", filepath.Join(constants.EFIMountPoint, "EFI", "boot"))
			}

			// list existing UKIs, and check if the current one is present
			ukiFiles, err := filepath.Glob(filepath.Join(constants.EFIMountPoint, "EFI", "Linux", "Talos-*.efi"))
			if err != nil {
				return err
			}

			for _, ukiFile := range ukiFiles {
				if strings.EqualFold(filepath.Base(ukiFile), bootedEntry) {
					config.Default = bootedEntry
				}
			}

			// here we handle a case when we boot of just kernel+initrd/uki and we don't have a booted entry
			if bootedEntry == "" && len(ukiFiles) == 1 {
				// we have only one UKI, so we can assume it's the default
				config.Default = filepath.Base(ukiFiles[0])
			}

			if callback != nil {
				return callback(config)
			}

			return nil
		},
		options.BlockProbeOptions,
		[]mountv2.NewPointOption{
			mountv2.WithReadonly(),
		},
		[]mountv2.OperationOption{
			mountv2.WithSkipIfMounted(),
		},
		nil,
	); err != nil {
		if xerrors.TagIs[mount.NotFoundTag](err) {
			return nil, nil
		}

		return nil, err
	}

	return config, nil
}

// Probe for existing sd-boot bootloader.
func Probe(disk string, options options.ProbeOptions) (*Config, error) {
	return ProbeWithCallback(disk, options, nil)
}

// KexecLoad does a kexec using the bootloader config.
//
//nolint:gocyclo
func (c *Config) KexecLoad(r runtime.Runtime, disk string) error {
	_, err := ProbeWithCallback(disk, options.ProbeOptions{}, func(conf *Config) error {
		var kernelFd int

		assetInfo, err := uki.Extract(filepath.Join(constants.EFIMountPoint, "EFI", "Linux", conf.Default))
		if err != nil {
			return fmt.Errorf("failed to extract kernel and initrd from uki: %w", err)
		}

		defer func() {
			if assetInfo.Closer != nil {
				assetInfo.Close() //nolint:errcheck
			}
		}()

		kernelFd, err = unix.MemfdCreate("vmlinux", 0)
		if err != nil {
			return fmt.Errorf("memfdCreate: %v", err)
		}

		kernelMemfd := os.NewFile(uintptr(kernelFd), "vmlinux")

		defer kernelMemfd.Close() //nolint:errcheck

		if _, err := io.Copy(kernelMemfd, assetInfo.Kernel); err != nil {
			return fmt.Errorf("failed to read kernel from uki: %w", err)
		}

		if _, err = kernelMemfd.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek kernel: %w", err)
		}

		initrdFd, err := unix.MemfdCreate("initrd", 0)
		if err != nil {
			return fmt.Errorf("memfdCreate: %v", err)
		}

		initrdMemfd := os.NewFile(uintptr(initrdFd), "initrd")

		defer initrdMemfd.Close() //nolint:errcheck

		if _, err := io.Copy(initrdMemfd, assetInfo.Initrd); err != nil {
			return fmt.Errorf("failed to read initrd from uki: %w", err)
		}

		if _, err = initrdMemfd.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek initrd: %w", err)
		}

		var cmdline strings.Builder

		if _, err := io.Copy(&cmdline, assetInfo.Cmdline); err != nil {
			return fmt.Errorf("failed to read cmdline from uki: %w", err)
		}

		if err := kexec.Load(r, kernelMemfd, initrdFd, cmdline.String()); err != nil {
			return fmt.Errorf("failed to load kernel for kexec: %w", err)
		}

		log.Printf("prepared kexec environment with kernel and initrd extracted from uki, cmdline=%q", cmdline.String())

		return nil
	})

	return err
}

// RequiredPartitions returns the list of partitions required by the bootloader.
func (c *Config) RequiredPartitions() []partition.Options {
	return []partition.Options{
		partition.NewPartitionOptions(constants.EFIPartitionLabel, true),
	}
}

// Install the bootloader.
func (c *Config) Install(opts options.InstallOptions) (*options.InstallResult, error) {
	var installResult *options.InstallResult

	err := mount.PartitionOp(
		opts.BootDisk,
		[]mount.Spec{
			{
				PartitionLabel: constants.EFIPartitionLabel,
				FilesystemType: partition.FilesystemTypeVFAT,
				MountTarget:    filepath.Join(opts.MountPrefix, constants.EFIMountPoint),
			},
		},
		func() error {
			var installErr error

			installResult, installErr = c.install(opts)

			return installErr
		},
		[]blkid.ProbeOption{
			// installation happens with locked blockdevice
			blkid.WithSkipLocking(true),
		},
		nil,
		nil,
		opts.BlkidInfo,
	)

	return installResult, err
}

// Install the bootloader.
//
// Assumes that EFI partition is already mounted.
// Writes down the UKI and updates the EFI variables.
//
//nolint:gocyclo
func (c *Config) install(opts options.InstallOptions) (*options.InstallResult, error) {
	var sdbootFilename string

	switch opts.Arch {
	case "amd64":
		sdbootFilename = "BOOTX64.efi"
	case "arm64":
		sdbootFilename = "BOOTAA64.efi"
	default:
		return nil, fmt.Errorf("unsupported architecture: %s", opts.Arch)
	}

	if _, err := os.Stat(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader", "loader.conf")); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader"), 0o755); err != nil {
				return nil, err
			}

			if err := os.WriteFile(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader", "loader.conf"), LoaderConfBytes, 0o644); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// list existing UKIs, and clean up all but the current one (used to boot)
	files, err := filepath.Glob(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", "Talos-*.efi"))
	if err != nil {
		return nil, err
	}

	// writing UKI by version-based filename here
	ukiPath := fmt.Sprintf("%s-%s.efi", "Talos", opts.Version)

	for _, file := range files {
		if strings.EqualFold(filepath.Base(file), c.Default) {
			if !strings.EqualFold(c.Default, ukiPath) {
				// set fallback to the current default unless it matches the new install
				c.Fallback = c.Default
			}

			continue
		}

		opts.Printf("removing old UKI: %s", file)

		if err = os.Remove(file); err != nil {
			return nil, err
		}
	}

	if err := utils.CopyFiles(
		opts.Printf,
		utils.SourceDestination(
			opts.BootAssets.UKIPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", ukiPath),
		),
		utils.SourceDestination(
			opts.BootAssets.SDBootPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "boot", sdbootFilename),
		),
	); err != nil {
		return nil, err
	}

	// don't update EFI variables if we're installing to a loop device
	if !opts.ImageMode {
		opts.Printf("updating EFI variables")

		efiCtx := efivario.NewDefaultContext()

		// set the new entry as a default one
		if err := WriteVariable(efiCtx, LoaderEntryDefaultName, ukiPath); err != nil {
			return nil, err
		}

		// set default 5 second boot timeout
		if err := WriteVariable(efiCtx, LoaderConfigTimeoutName, "5"); err != nil {
			return nil, err
		}
	}

	if opts.ExtraInstallStep != nil {
		if err := opts.ExtraInstallStep(); err != nil {
			return nil, err
		}
	}

	return &options.InstallResult{
		PreviousLabel: c.Fallback,
	}, nil
}

// Revert the bootloader to the previous version.
func (c *Config) Revert(disk string) error {
	err := mount.PartitionOp(
		disk,
		[]mount.Spec{
			{
				PartitionLabel: constants.EFIPartitionLabel,
				FilesystemType: partition.FilesystemTypeVFAT,
				MountTarget:    constants.EFIMountPoint,
			},
		},
		c.revert,
		nil,
		nil,
		[]mountv2.OperationOption{
			mountv2.WithSkipIfMounted(),
		},
		nil,
	)
	if err != nil && !xerrors.TagIs[mount.NotFoundTag](err) {
		return err
	}

	return nil
}

func (c *Config) revert() error {
	files, err := filepath.Glob(filepath.Join(constants.EFIMountPoint, "EFI", "Linux", "Talos-*.efi"))
	if err != nil {
		return err
	}

	for _, file := range files {
		if strings.EqualFold(filepath.Base(file), c.Default) {
			continue
		}

		log.Printf("reverting to previous UKI: %s", file)

		return WriteVariable(efivario.NewDefaultContext(), LoaderEntryDefaultName, filepath.Base(file))
	}

	return errors.New("previous UKI not found")
}
