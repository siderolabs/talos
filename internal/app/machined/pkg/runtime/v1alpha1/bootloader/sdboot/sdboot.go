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
	"strconv"
	"strings"

	"github.com/foxboron/go-uefi/efi"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/partition"
	smbiosinternal "github.com/siderolabs/talos/internal/pkg/smbios"
	"github.com/siderolabs/talos/internal/pkg/uki"
	"github.com/siderolabs/talos/pkg/imager/utils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
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
// this is called when we upgrade, do KexecLoad, or for reverting the bootloader.
//
//nolint:gocyclo
func ProbeWithCallback(disk string, options options.ProbeOptions, callback func(*Config) error) (*Config, error) {
	// if not UEFI boot, nothing to do
	if !IsUEFIBoot() {
		options.Log("sd-boot: not booted using UEFI, skipping probing")

		return nil, nil
	}

	var sdbootConf *Config

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
				return fmt.Errorf("no boot*.efi files found in %s", filepath.Join(constants.EFIMountPoint, "EFI", "boot"))
			}

			// list existing UKIs, and check if the current one is present
			ukiFiles, err := filepath.Glob(filepath.Join(constants.EFIMountPoint, "EFI", "Linux", "Talos-*.efi"))
			if err != nil {
				return err
			}

			if len(ukiFiles) == 0 {
				return fmt.Errorf("no UKI files found in %q", filepath.Join(constants.EFIMountPoint, "EFI", "Linux"))
			}

			options.Log("sd-boot: found UKI files: %v", xslices.Map(ukiFiles, filepath.Base))

			// If we booted of UKI/Kernel+Initramfs/ISO Talos installer will always be run which
			// sets the `LoaderEntryDefault` to the UKI file name, so either for reboot with Kexec or upgrade
			// we will always have the UKI file name in the `LoaderEntryDefault`
			// and we can use it to determine the default entry.
			bootEntry, err := ReadVariable(LoaderEntryDefaultName)
			if err != nil {
				return err
			}

			options.Log("sd-boot: LoaderEntryDefault: %s", bootEntry)

			if bootEntry == "" {
				// If we booted of a Disk image, only `LoaderEntrySelected` will be set until we do an upgrade
				// which will set the `LoaderEntryDefault` to the UKI file name.
				// So for reboot with Kexec we will have to read the `LoaderEntrySelected`
				// upgrades will always have `LoaderEntryDefault` set to the UKI file name.
				loaderEntrySelected, err := ReadVariable(LoaderEntrySelectedName)
				if err != nil {
					return err
				}

				if loaderEntrySelected == "" {
					return errors.New("sd-boot: no LoaderEntryDefault or LoaderEntrySelected found, cannot continue")
				}

				bootEntry = loaderEntrySelected
			}

			options.Log("sd-boot: found boot entry: %s", bootEntry)

			for _, ukiFile := range ukiFiles {
				if strings.EqualFold(filepath.Base(ukiFile), bootEntry) {
					options.Log("sd-boot: default entry matched as %q", bootEntry)

					sdbootConf = &Config{
						Default: bootEntry,
					}
				}
			}

			if sdbootConf == nil {
				return errors.New("sd-boot: no valid sd-boot config found, cannot continue")
			}

			options.Log("sd-boot: using %s as default entry", sdbootConf.Default)

			if callback != nil {
				return callback(sdbootConf)
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

	return sdbootConf, nil
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

		if !efi.GetSecureBoot() {
			smbiosInfo, err := smbiosinternal.GetSMBIOSInfo()
			if err == nil {
				for _, structure := range smbiosInfo.Structures {
					if structure.Header.Type != 11 {
						continue
					}

					const kernelCmdlineExtra = "io.systemd.stub.kernel-cmdline-extra="

					for _, s := range structure.Strings {
						if strings.HasPrefix(s, kernelCmdlineExtra) {
							cmdline.WriteString(" " + s[len(kernelCmdlineExtra):])
						}
					}
				}
			}
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
func (c *Config) RequiredPartitions(quirk quirks.Quirks) []partition.Options {
	return []partition.Options{
		partition.NewPartitionOptions(constants.EFIPartitionLabel, true, quirk),
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

func sdBootFilePath(arch string) (string, error) {
	basePath := filepath.Join("EFI", "boot")

	switch arch {
	case "amd64":
		return filepath.Join(basePath, "BOOTX64.efi"), nil
	case "arm64":
		return filepath.Join(basePath, "BOOTAA64.efi"), nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}

// Install the bootloader.
//
// Assumes that EFI partition is already mounted.
// Writes down the UKI and updates the EFI variables.
//
//nolint:gocyclo,cyclop
func (c *Config) install(opts options.InstallOptions) (*options.InstallResult, error) {
	if _, err := os.Stat(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader", "loader.conf")); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}

		if err := os.MkdirAll(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader"), 0o755); err != nil {
			return nil, err
		}

		if err := os.WriteFile(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader", "loader.conf"), LoaderConfBytes, 0o644); err != nil {
			return nil, err
		}
	}

	// list existing UKIs, and clean up all but the current one (used to boot)
	files, err := filepath.Glob(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", "Talos-*.efi"))
	if err != nil {
		return nil, err
	}

	opts.Printf("sd-boot: found existing UKIs during install: %v", xslices.Map(files, filepath.Base))

	ukiPath, err := GenerateNextUKIName(opts.Version, files)
	if err != nil {
		return nil, fmt.Errorf("failed to generate next UKI name: %w", err)
	}

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

	sdbootFilename, err := sdBootFilePath(opts.Arch)
	if err != nil {
		return nil, fmt.Errorf("failed to get sd-boot file path: %w", err)
	}

	if err := utils.CopyFiles(
		opts.Printf,
		utils.SourceDestination(
			opts.BootAssets.UKIPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", ukiPath),
		),
		utils.SourceDestination(
			opts.BootAssets.SDBootPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, sdbootFilename),
		),
	); err != nil {
		return nil, err
	}

	// don't update EFI variables if we're installing to a loop device
	if !opts.ImageMode {
		opts.Printf("updating EFI variables")

		// set the new entry as a default one
		if err := WriteVariable(LoaderEntryDefaultName, ukiPath); err != nil {
			return nil, err
		}

		// set default 5 second boot timeout
		if err := WriteVariable(LoaderConfigTimeoutName, "5"); err != nil {
			return nil, err
		}

		if err := CreateBootEntry(opts.BootDisk, sdbootFilename); err != nil {
			return nil, fmt.Errorf("failed to create boot entry: %w", err)
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

// GenerateNextUKIName generates the next UKI name based on the version and existing files.
// It checks for existing files and increments the index if necessary.
func GenerateNextUKIName(version string, existingFiles []string) (string, error) {
	maxIndex := -1

	for _, file := range existingFiles {
		base := strings.TrimSuffix(filepath.Base(file), ".efi")
		if !strings.HasPrefix(base, "Talos-") {
			continue
		}

		suffix := strings.TrimPrefix(base, "Talos-")
		parts := strings.SplitN(suffix, "~", 2)

		if parts[0] != version {
			continue
		}

		if len(parts) == 1 {
			// Talos-{version}.efi format
			if maxIndex < 0 {
				maxIndex = 0
			}
		} else if len(parts) == 2 {
			// Talos-{version}+{index}.efi format
			if idx, err := strconv.Atoi(parts[1]); err == nil && idx > maxIndex {
				maxIndex = idx
			}
		}
	}

	if maxIndex >= 0 {
		return fmt.Sprintf("Talos-%s~%d.efi", version, maxIndex+1), nil
	}

	return fmt.Sprintf("Talos-%s.efi", version), nil
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

		return WriteVariable(LoaderEntryDefaultName, filepath.Base(file))
	}

	return errors.New("previous UKI not found")
}
