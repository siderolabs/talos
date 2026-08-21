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
	"slices"
	"strconv"
	"strings"

	"github.com/foxboron/go-uefi/efi"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	bootloaderutils "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/efiutils"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/kexec"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/efivarfs"
	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/internal/pkg/smbios"
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
	// BootedEntry is the UKI the system is running from right now.
	//
	// This is what the installer needs on upgrade, to keep the running generation around as a
	// fallback.
	BootedEntry string
	// NextBootEntry is the UKI sd-boot picks on the next boot, i.e. LoaderEntryDefault.
	//
	// This is what kexec needs to load, as kexec replaces a firmware boot which would have been
	// handled by sd-boot, and what reverting moves the default away from, as reverting should
	// change the entry which boots next.
	NextBootEntry string
	// Fallback is the previous entry kept around by an upgrade.
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
// The Config carries both the entry the system is running from (Config.BootedEntry) and the entry
// which boots next (Config.NextBootEntry): those differ e.g. right after the installer updated
// LoaderEntryDefault, or when an operator selected a non-default entry in the sd-boot menu.
//
//nolint:gocyclo
func ProbeWithCallback(disk string, options options.ProbeOptions, callback func(*Config) error) (*Config, error) {
	// if not UEFI boot, nothing to do
	if !IsUEFIBoot() {
		options.Logf("sd-boot: not booted using UEFI, skipping probing")

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

			options.Logf("sd-boot: found UKI files: %v", xslices.Map(ukiFiles, filepath.Base))

			// LoaderEntryDefault is the persistent sd-boot default. It is not necessarily the
			// entry running now, because an operator can select a different entry at boot.
			loaderEntryDefault, err := ReadVariable(LoaderEntryDefaultName)
			if err != nil {
				return err
			}

			options.Logf("sd-boot: LoaderEntryDefault: %s", loaderEntryDefault)

			// LoaderEntrySelected is the entry selected by sd-boot for this boot. It can be
			// stale after kexec, where sd-boot did not participate in the current boot.
			loaderEntrySelected, err := ReadVariable(LoaderEntrySelectedName)
			if err != nil {
				return err
			}

			options.Logf("sd-boot: LoaderEntrySelected: %s", loaderEntrySelected)

			loaderEntryOneShot, err := ReadVariable(LoaderEntryOneShotName)
			if err != nil {
				return err
			}

			options.Logf("sd-boot: LoaderEntryOneShot: %s", loaderEntryOneShot)

			loaderEntryRebootReason, err := ReadVariable(LoaderEntryRebootReasonName)
			if err != nil {
				return err
			}

			options.Logf("sd-boot: LoaderEntryRebootReason: %s", loaderEntryRebootReason)

			if loaderEntrySelected == "" && loaderEntryDefault == "" {
				return errors.New("sd-boot: no LoaderEntryDefault or LoaderEntrySelected found, cannot continue")
			}

			bootedEntry, bootedEntryOk := findBootedUKIFile(
				ukiFiles,
				loaderEntryDefault,
				loaderEntrySelected,
				loaderEntryOneShot,
				loaderEntryRebootReason,
			)

			nextBootEntry, nextBootEntryOk := findNextBootUKIFile(ukiFiles, loaderEntryDefault, loaderEntrySelected)

			if !bootedEntryOk || !nextBootEntryOk {
				return errors.New("sd-boot: no valid boot entry found matching LoaderEntrySelected or LoaderEntryDefault")
			}

			sdbootConf = &Config{
				BootedEntry:   bootedEntry,
				NextBootEntry: nextBootEntry,
			}

			options.Logf("sd-boot: booted entry: %s, next boot entry: %s", sdbootConf.BootedEntry, sdbootConf.NextBootEntry)

			if callback != nil {
				return callback(sdbootConf)
			}

			return nil
		},
		options.BlockProbeOptions,
		[]mountv3.ManagerOption{
			mountv3.WithSkipIfMounted(),
			mountv3.WithReadOnly(),
		},
		nil,
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

		// kexec replaces the next firmware boot, so load the entry sd-boot would have booted,
		// and not the one which is running right now.
		assetInfo, err := uki.Extract(filepath.Join(constants.EFIMountPoint, "EFI", "Linux", conf.NextBootEntry))
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
			if extraCmdline, err := smbios.ReadOEMVariable(constants.SDStubCmdlineExtraOEMVar); err == nil {
				for _, s := range extraCmdline {
					cmdline.WriteString(" " + s)
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

// PrepareBootPartitions prepares the set of partitions to create for the bootloader.
//
// In image mode, this also pre-populates the assets to be written to the bootloader partitions
// when formatting the filesystem.
// In install mode, this only returns a list of partitions to create, and the assets are written to the partitions during Install.
func (c *Config) PrepareBootPartitions(opts options.InstallOptions) ([]partition.Options, error) {
	quirk := quirks.New(opts.Version)

	formatOptions := []partition.FormatOption{
		partition.WithLabel(constants.EFIPartitionLabel),
	}

	if opts.ImageMode {
		formatOptions = append(formatOptions, partition.WithSourceDirectory(filepath.Join(opts.MountPrefix, "EFI")))
	}

	partitionOptions := []partition.Options{
		partition.NewPartitionOptions(
			true,
			quirk,
			formatOptions...,
		),
	}

	if !opts.ImageMode {
		// in non-image mode, the actual asset copying will happen in the Install step
		return partitionOptions, nil
	}

	// in image mode, we populate the directory with the bootloader assets
	ukiFileName, err := generateNextUKIName(opts.Version, nil)
	if err != nil {
		return nil, err
	}

	if err := c.copyAssets(opts, ukiFileName); err != nil {
		return nil, err
	}

	// move the files out of constants.EFIMountPoint into the "EFI" directory to prevent it being pulled into the GRUB /boot partition in dual-boot mode
	if err := os.Rename(filepath.Join(opts.MountPrefix, constants.EFIMountPoint), filepath.Join(opts.MountPrefix, "EFI")); err != nil {
		return nil, fmt.Errorf("failed to move EFI directory: %w", err)
	}

	partitionOptions = xslices.Map(partitionOptions, func(o partition.Options) partition.Options {
		o.Reproducible = true

		return o
	})

	return partitionOptions, nil
}

// Install the bootloader.
//
// This method only runs in non-image mode, so mount the EFI partition, copy the UKI and sd-boot.efi, and update the EFI variables.
func (c *Config) Install(opts options.InstallOptions) (*options.InstallResult, error) {
	ukiFileName, err := generateNextUKIName(opts.Version, nil)
	if err != nil {
		return nil, err
	}

	var installResult *options.InstallResult

	err = mount.PartitionOp(
		opts.BootDisk,
		[]mount.Spec{
			{
				PartitionLabel: constants.EFIPartitionLabel,
				FilesystemType: partition.FilesystemTypeVFAT,
				MountTarget:    filepath.Join(opts.MountPrefix, constants.EFIMountPoint),
			},
		},
		func() error {
			if err := c.copyAssets(opts, ukiFileName); err != nil {
				return err
			}

			var err error

			installResult, err = c.setup(opts, ukiFileName)
			if err != nil {
				return err
			}

			return nil
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

// Upgrade the bootloader.
// On upgrade we mount the EFI partition, cleanup old UKIs, copy the new UKI and sd-boot.efi, and update the EFI variables.
//
//nolint:gocyclo
func (c *Config) Upgrade(opts options.InstallOptions) (*options.InstallResult, error) {
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
			// list existing UKIs, and clean up all but the current one (used to boot)
			files, err := filepath.Glob(filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", "Talos-*.efi"))
			if err != nil {
				return err
			}

			opts.Printf("sd-boot: found existing UKIs during upgrade: %v", xslices.Map(files, filepath.Base))

			ukiPath, err := generateNextUKIName(opts.Version, files)
			if err != nil {
				return fmt.Errorf("failed to generate next UKI name: %w", err)
			}

			// the cleanup below might remove the UKI LoaderEntryDefault points to (when the entry which
			// boots next is not the one we are running from), so repoint the default to the booted entry
			// first: if the upgrade is interrupted before setup() rewrites it, the next boot still lands
			// on a UKI which exists.
			if !strings.EqualFold(c.NextBootEntry, c.BootedEntry) {
				opts.Printf("sd-boot: pointing the default entry to the booted one for the duration of the upgrade: %s", c.BootedEntry)

				if err = WriteVariable(LoaderEntryDefaultName, c.BootedEntry); err != nil {
					return err
				}
			}

			for _, file := range files {
				if strings.EqualFold(filepath.Base(file), c.BootedEntry) {
					if !strings.EqualFold(c.BootedEntry, ukiPath) {
						// set fallback to the entry we are running from unless it matches the new install
						c.Fallback = c.BootedEntry
					}

					continue
				}

				opts.Printf("removing old UKI: %s", file)

				if err = os.Remove(file); err != nil {
					return err
				}
			}

			if err := c.copyAssets(opts, ukiPath); err != nil {
				return err
			}

			installResult, err = c.setup(opts, ukiPath)
			if err != nil {
				return err
			}

			return nil
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
//nolint:gocyclo,cyclop
func (c *Config) setup(opts options.InstallOptions, ukiFileName string) (*options.InstallResult, error) {
	opts.Printf("updating EFI variables")

	// set the new entry as a default one
	if err := WriteVariable(LoaderEntryDefaultName, ukiFileName); err != nil {
		return nil, err
	}

	// set default 5 second boot timeout
	if err := WriteVariable(LoaderConfigTimeoutName, "5"); err != nil {
		return nil, err
	}

	efiRW, err := efivarfs.NewFilesystemReaderWriter(true)
	if err != nil {
		return nil, fmt.Errorf("failed to create efivarfs reader/writer: %w", err)
	}

	defer efiRW.Close() //nolint:errcheck

	blkidInfo, err := blkid.ProbePath(opts.BootDisk, blkid.WithSkipLocking(true))
	if err != nil {
		return nil, fmt.Errorf("failed to probe block device %s: %w", opts.BootDisk, err)
	}

	sdbootFilename, err := bootloaderutils.Name(opts.Arch)
	if err != nil {
		return nil, fmt.Errorf("failed to get sd-boot file path: %w", err)
	}

	if err := CreateBootEntry(efiRW, blkidInfo, opts.Printf, sdbootFilename); err != nil {
		return nil, fmt.Errorf("failed to create boot entry: %w", err)
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

func (c *Config) copyAssets(opts options.InstallOptions, ukiFileName string) error {
	loaderDir := filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "loader")

	if err := os.MkdirAll(loaderDir, 0o755); err != nil {
		return err
	}

	loaderConf := LoaderConfBytes

	if opts.SecureBootEnrollKeys != "" {
		loaderConf = slices.Concat(loaderConf, []byte("\nsecure-boot-enroll "+opts.SecureBootEnrollKeys+"\n"))

		keysDir := filepath.Join(loaderDir, "keys", "auto")
		if err := os.MkdirAll(keysDir, 0o755); err != nil {
			return err
		}

		if err := utils.CopyFiles(
			opts.Printf,
			utils.SourceDestination(opts.PlatformKeyPath, filepath.Join(keysDir, constants.PlatformKeyAsset)),
			utils.SourceDestination(opts.KeyExchangeKeyPath, filepath.Join(keysDir, constants.KeyExchangeKeyAsset)),
			utils.SourceDestination(opts.SignatureKeyPath, filepath.Join(keysDir, constants.SignatureKeyAsset)),
		); err != nil {
			return err
		}
	}

	if err := os.WriteFile(filepath.Join(loaderDir, "loader.conf"), loaderConf, 0o644); err != nil {
		return err
	}

	sdbootFilename, err := bootloaderutils.Name(opts.Arch)
	if err != nil {
		return fmt.Errorf("failed to get sd-boot file path: %w", err)
	}

	if err := utils.CopyFiles(
		opts.Printf,
		utils.SourceDestination(
			opts.BootAssets.UKIPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, "EFI", "Linux", ukiFileName),
		),
		utils.SourceDestination(
			opts.BootAssets.SDBootPath,
			filepath.Join(opts.MountPrefix, constants.EFIMountPoint, sdbootFilename),
		),
	); err != nil {
		return err
	}

	return nil
}

// generateNextUKIName generates the next UKI name based on the version and existing files.
// It checks for existing files and increments the index if necessary.
func generateNextUKIName(version string, existingFiles []string) (string, error) {
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
		[]mountv3.ManagerOption{
			mountv3.WithSkipIfMounted(),
		},
		nil,
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
		// skip the entry which boots next, as the whole point of the revert is to boot something else
		if strings.EqualFold(filepath.Base(file), c.NextBootEntry) {
			continue
		}

		log.Printf("reverting to previous UKI: %s", file)

		return WriteVariable(LoaderEntryDefaultName, filepath.Base(file))
	}

	return errors.New("previous UKI not found")
}

func findMatchingUKIFile(ukiFiles []string, entry string) (string, bool) {
	if slices.ContainsFunc(ukiFiles, func(file string) bool {
		return strings.EqualFold(filepath.Base(file), entry)
	}) {
		return entry, true
	}

	return "", false
}

// findBootedUKIFile returns the UKI the system is running from.
func findBootedUKIFile(ukiFiles []string, defaultEntry, selectedEntry, oneShotEntry, rebootReason string) (string, bool) {
	// A kexec reboot bypasses sd-boot, so LoaderEntrySelected can describe an older
	// boot. In that case the installer-updated default is the running entry.
	if rebootReason == "reboot" && oneShotEntry == "kexec reboot" {
		if entry, ok := findMatchingUKIFile(ukiFiles, defaultEntry); ok {
			return entry, true
		}

		return findMatchingUKIFile(ukiFiles, selectedEntry)
	}

	// For a firmware boot, LoaderEntrySelected is the entry actually running and
	// may intentionally differ from the persistent default after manual selection.
	if entry, ok := findMatchingUKIFile(ukiFiles, selectedEntry); ok {
		return entry, true
	}

	return findMatchingUKIFile(ukiFiles, defaultEntry)
}

// findNextBootUKIFile returns the UKI sd-boot boots next, which is the persistent default.
//
// LoaderEntrySelected is only a fallback for a system which never ran the installer (e.g. booted
// off a disk image), as there LoaderEntryDefault is not set yet.
func findNextBootUKIFile(ukiFiles []string, defaultEntry, selectedEntry string) (string, bool) {
	if entry, ok := findMatchingUKIFile(ukiFiles, defaultEntry); ok {
		return entry, true
	}

	return findMatchingUKIFile(ukiFiles, selectedEntry)
}
