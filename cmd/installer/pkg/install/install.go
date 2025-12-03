// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package install provides the installation routine.
package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/uuid"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	bootloaderoptions "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/imager/overlay/executor"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/overlay"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// Options represents the set of options available for an install.
type Options struct {
	ConfigSource string
	// Can be an actual disk path or a file representing a disk image.
	DiskPath            string
	Platform            string
	Arch                string
	Board               string
	ExtraKernelArgs     []string
	Upgrade             bool
	Force               bool
	Zero                bool
	LegacyBIOSSupport   bool
	GrubUseUKICmdline   bool
	MetaValues          MetaValues
	OverlayInstaller    overlay.Installer[overlay.ExtraOptions]
	OverlayName         string
	OverlayExtractedDir string
	ExtraOptions        overlay.ExtraOptions

	ImageCachePath string

	// Options specific for the image creation mode.
	ImageSecureboot     bool
	DiskImageBootloader string
	Version             string
	BootAssets          bootloaderoptions.BootAssets
	Printf              func(string, ...any)
	MountPrefix         string
}

// Mode is the install mode.
type Mode int

const (
	// ModeInstall is the install mode.
	ModeInstall Mode = iota
	// ModeUpgrade is the upgrade mode.
	ModeUpgrade
	// ModeImage is the image creation mode.
	ModeImage
)

// IsImage returns true if the mode is image creation.
func (m Mode) IsImage() bool {
	return m == ModeImage
}

const typeGPT = "gpt"

// Install installs Talos.
//
//nolint:gocyclo
func Install(ctx context.Context, p runtime.Platform, mode Mode, opts *Options) error {
	overlayPresent := overlayPresent()

	if b := getBoard(); b != constants.BoardNone && !overlayPresent {
		return fmt.Errorf("using standard installer image is not supported for board: %s, use an installer with overlay", b)
	}

	if overlayPresent {
		extraOptionsBytes, err := os.ReadFile(constants.ImagerOverlayExtraOptionsPath)
		if err != nil {
			return err
		}

		var extraOptions overlay.ExtraOptions

		decoder := yaml.NewDecoder(bytes.NewReader(extraOptionsBytes))
		decoder.KnownFields(true)

		if err := decoder.Decode(&extraOptions); err != nil {
			return fmt.Errorf("failed to decode extra options: %w", err)
		}

		opts.OverlayInstaller = executor.New(constants.ImagerOverlayInstallerDefaultPath)
		opts.ExtraOptions = extraOptions
	}

	// NOTE: this is legacy code which is only used when running in GRUB mode with GrubUseUKICmdline set to false.
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamPlatform, p.Name())

	if opts.ConfigSource != "" {
		cmdline.Append(constants.KernelParamConfig, opts.ConfigSource)
	}

	cmdline.SetAll(p.KernelArgs(opts.Arch, quirks.Quirks{}).Strings())

	// first defaults, then extra kernel args to allow extra kernel args to override defaults
	if err := cmdline.AppendAll(kernel.DefaultArgs(quirks.Quirks{})); err != nil {
		return err
	}

	if opts.Board != constants.BoardNone {
		// board 'rpi_4' was removed in Talos 1.5 in favor of `rpi_generic`
		if opts.Board == "rpi_4" {
			opts.Board = constants.BoardRPiGeneric
		}

		var b runtime.Board

		b, err := board.NewBoard(opts.Board) //nolint:staticcheck
		if err != nil {
			return err
		}

		cmdline.Append(constants.KernelParamBoard, b.Name())

		cmdline.SetAll(b.KernelArgs().Strings())
	}

	if opts.OverlayInstaller != nil {
		overlayOpts, getOptsErr := opts.OverlayInstaller.GetOptions(ctx, opts.ExtraOptions)
		if getOptsErr != nil {
			return fmt.Errorf("failed to get overlay installer options: %w", getOptsErr)
		}

		opts.OverlayName = overlayOpts.Name

		cmdline.SetAll(overlayOpts.KernelArgs)
	}

	// preserve console=ttyS0 if it was already present in cmdline for metal platform
	existingCmdline := procfs.ProcCmdline()

	if *existingCmdline.Get(constants.KernelParamPlatform).First() == constants.PlatformMetal && existingCmdline.Get("console").Contains("ttyS0") {
		if !slices.Contains(opts.ExtraKernelArgs, "console=ttyS0") {
			cmdline.Append("console", "ttyS0")
		}
	}

	if err := cmdline.AppendAll(
		opts.ExtraKernelArgs,
		procfs.WithOverwriteArgs("console"),
		procfs.WithOverwriteArgs(constants.KernelParamPlatform),
		procfs.WithDeleteNegatedArgs(),
	); err != nil {
		return err
	}

	i, err := NewInstaller(ctx, cmdline, mode, opts)
	if err != nil {
		return err
	}

	if err = i.Install(ctx, mode); err != nil {
		return err
	}

	i.options.Printf("installation of %s complete", version.Tag)

	return nil
}

// Installer represents the installer logic. It serves as the entrypoint to all
// installation methods.
type Installer struct {
	cmdline *procfs.Cmdline
	options *Options
}

// NewInstaller initializes and returns an Installer.
func NewInstaller(ctx context.Context, cmdline *procfs.Cmdline, mode Mode, opts *Options) (i *Installer, err error) {
	i = &Installer{
		cmdline: cmdline,
		options: opts,
	}

	if i.options.Version == "" {
		i.options.Version = version.Tag
	}

	if i.options.Printf == nil {
		i.options.Printf = log.Printf
	}

	if mode == ModeUpgrade && i.options.Force {
		i.options.Printf("system disk wipe on upgrade is not supported anymore, option ignored")
	}

	if i.options.Zero && mode != ModeInstall {
		i.options.Printf("zeroing of the disk is only supported for the initial installation, option ignored")
	}

	i.options.BootAssets.FillDefaults(opts.Arch)

	return i, nil
}

// detectBootloader detects the bootloader to use based on the mode.
func (i *Installer) detectBootloader(mode Mode) (bootloader.Bootloader, error) {
	switch mode {
	case ModeInstall:
		return bootloader.NewAuto(), nil
	case ModeUpgrade:
		return bootloader.Probe(i.options.DiskPath, bootloaderoptions.ProbeOptions{
			// the disk is already locked
			BlockProbeOptions: []blkid.ProbeOption{
				blkid.WithSkipLocking(true),
			},
			Logger: log.Printf,
		})
	case ModeImage:
		return bootloader.New(i.options.DiskImageBootloader, i.options.Version, i.options.Arch)
	default:
		return nil, fmt.Errorf("unknown image mode: %d", mode)
	}
}

// diskOperations performs any disk operations required before installation.
//
//nolint:gocyclo
func (i *Installer) diskOperations(mode Mode) error {
	switch mode {
	case ModeInstall:
		bd, info, err := i.blockDeviceData()
		if err != nil {
			return fmt.Errorf("failed to access blockdevice %s: %w", i.options.DiskPath, err)
		}

		defer bd.Close()  //nolint:errcheck
		defer bd.Unlock() //nolint:errcheck

		if !i.options.Zero && !i.options.Force {
			// verify that the disk is either empty or has an empty GPT partition table, otherwise fail the install
			switch {
			case info.Name == "":
				// empty, ok
			case info.Name == typeGPT && len(info.Parts) == 0:
				// GPT, no partitions, ok
			default:
				return fmt.Errorf("disk %s is not empty, skipping install, detected %q", i.options.DiskPath, info.Name)
			}
		} else {
			// zero the disk
			if err = bd.FastWipe(); err != nil {
				return fmt.Errorf("failed to zero blockdevice %s: %w", i.options.DiskPath, err)
			}
		}

		return nil
	case ModeUpgrade:
		// if running in upgrade mode, verify that install disk has correct GPT partition table and expected partitions
		bd, info, err := i.blockDeviceData()
		if err != nil {
			return fmt.Errorf("failed to access blockdevice %s: %w", i.options.DiskPath, err)
		}

		defer bd.Close()  //nolint:errcheck
		defer bd.Unlock() //nolint:errcheck

		// on upgrade, we don't touch the disk partitions, but we need to verify that the disk has the expected GPT partition table
		if info.Name != typeGPT {
			return fmt.Errorf("disk %s has an unexpected format %q", i.options.DiskPath, info.Name)
		}

		return nil
	case ModeImage:
		// no disk operations required for image creation
		return nil
	default:
		return fmt.Errorf("unknown image mode: %d", mode)
	}
}

// blockDeviceData opens and locks the block device and probes it.
// the caller is responsible for unlocking and closing the block device.
func (i *Installer) blockDeviceData() (*block.Device, *blkid.Info, error) {
	// open and lock the blockdevice for the installation disk for the whole duration of the installer
	bd, err := block.NewFromPath(i.options.DiskPath, block.OpenForWrite())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open blockdevice %s: %w", i.options.DiskPath, err)
	}

	defer bd.Close() //nolint:errcheck

	if err = bd.Lock(true); err != nil {
		return nil, nil, fmt.Errorf("failed to lock blockdevice %s: %w", i.options.DiskPath, err)
	}

	defer bd.Unlock() //nolint:errcheck

	// probe the disk anyways
	info, err := blkid.Probe(bd.File(), blkid.WithSkipLocking(true))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to probe blockdevice %s: %w", i.options.DiskPath, err)
	}

	return bd, info, nil
}

// Install fetches the necessary data locations and copies or extracts
// to the target locations.
//
//nolint:gocyclo,cyclop
func (i *Installer) Install(ctx context.Context, mode Mode) (err error) {
	// pre-flight checks, erratas
	hostTalosVersion, err := readHostTalosVersion()
	if err != nil {
		return err
	}

	if mode == ModeUpgrade {
		i.errataNetIfnames(hostTalosVersion)
	}

	if err = i.runPreflightChecks(mode); err != nil {
		return err
	}

	bootlder, err := i.detectBootloader(mode)
	if err != nil {
		return fmt.Errorf("failed to detect bootloader: %w", err)
	}

	if err = i.diskOperations(mode); err != nil {
		return fmt.Errorf("failed to perform disk operations: %w", err)
	}

	// create partitions and re-probe the device
	info, err := i.createPartitions(ctx, mode, hostTalosVersion, bootlder)
	if err != nil {
		return fmt.Errorf("failed to create partitions: %w", err)
	}

	// Install the bootloader.
	bootInstallResult, err := bootlder.Install(bootloaderoptions.InstallOptions{
		BootDisk:          i.options.DiskPath,
		Arch:              i.options.Arch,
		Cmdline:           i.cmdline.String(),
		GrubUseUKICmdline: i.options.GrubUseUKICmdline,
		Version:           i.options.Version,
		ImageMode:         mode.IsImage(),
		MountPrefix:       i.options.MountPrefix,
		BootAssets:        i.options.BootAssets,
		Printf:            i.options.Printf,
		BlkidInfo:         info,

		ExtraInstallStep: func() error {
			if i.options.Board != constants.BoardNone {
				var b runtime.Board

				b, err = board.NewBoard(i.options.Board) //nolint:staticcheck
				if err != nil {
					return err
				}

				i.options.Printf("installing U-Boot for %q", b.Name())

				if err = b.Install(runtime.BoardInstallOptions{
					InstallDisk:     i.options.DiskPath,
					MountPrefix:     i.options.MountPrefix,
					UBootPath:       i.options.BootAssets.UBootPath,
					DTBPath:         i.options.BootAssets.DTBPath,
					RPiFirmwarePath: i.options.BootAssets.RPiFirmwarePath,
					Printf:          i.options.Printf,
				}); err != nil {
					return fmt.Errorf("failed to install for board %s: %w", b.Name(), err)
				}
			}

			if i.options.OverlayInstaller != nil {
				i.options.Printf("running overlay installer %q", i.options.OverlayName)

				if err = i.options.OverlayInstaller.Install(ctx, overlay.InstallOptions[overlay.ExtraOptions]{
					InstallDisk:   i.options.DiskPath,
					MountPrefix:   i.options.MountPrefix,
					ArtifactsPath: filepath.Join(i.options.OverlayExtractedDir, constants.ImagerOverlayArtifactsPath),
					ExtraOptions:  i.options.ExtraOptions,
				}); err != nil {
					return fmt.Errorf("failed to run overlay installer: %w", err)
				}
			}

			return nil
		},
	})
	if err != nil {
		return fmt.Errorf("failed to install bootloader: %w", err)
	}

	if mode == ModeUpgrade || len(i.options.MetaValues.values) > 0 {
		var (
			metaState         *meta.Meta
			metaPartitionName string
		)

		for _, partition := range info.Parts {
			if pointer.SafeDeref(partition.PartitionLabel) == constants.MetaPartitionLabel {
				metaPartitionName = partitioning.DevName(i.options.DiskPath, partition.PartitionIndex)

				break
			}
		}

		if metaPartitionName == "" {
			return errors.New("failed to detect META partition")
		}

		if metaState, err = meta.New(ctx, nil, meta.WithPrinter(i.options.Printf), meta.WithFixedPath(metaPartitionName)); err != nil {
			return fmt.Errorf("failed to open META: %w", err)
		}

		var ok bool

		if mode == ModeUpgrade {
			if ok, err = metaState.SetTag(ctx, metaconsts.Upgrade, bootInstallResult.PreviousLabel); !ok || err != nil {
				return fmt.Errorf("failed to set upgrade tag: %q", bootInstallResult.PreviousLabel)
			}
		}

		for _, v := range i.options.MetaValues.values {
			if ok, err = metaState.SetTag(ctx, v.Key, v.Value); !ok || err != nil {
				return fmt.Errorf("failed to set meta tag: %q -> %q", v.Key, v.Value)
			}
		}

		if err = metaState.Flush(); err != nil {
			return fmt.Errorf("failed to flush META: %w", err)
		}
	}

	return nil
}

//nolint:gocyclo,cyclop
func (i *Installer) createPartitions(ctx context.Context, mode Mode, hostTalosVersion *compatibility.TalosVersion, bootlder bootloader.Bootloader) (*blkid.Info, error) {
	var gptdev gpt.Device

	switch mode {
	case ModeInstall, ModeImage:
		bd, _, err := i.blockDeviceData()
		if err != nil {
			return nil, fmt.Errorf("failed to access blockdevice %s: %w", i.options.DiskPath, err)
		}

		defer bd.Close()  //nolint:errcheck
		defer bd.Unlock() //nolint:errcheck

		gptdev, err = gpt.DeviceFromBlockDevice(bd)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize GPT device from blockdevice %s: %w", i.options.DiskPath, err)
		}
	case ModeUpgrade:
		bd, info, err := i.blockDeviceData()
		if err != nil {
			return nil, fmt.Errorf("failed to access blockdevice %s: %w", i.options.DiskPath, err)
		}

		defer bd.Close()  //nolint:errcheck
		defer bd.Unlock() //nolint:errcheck

		return info, nil
	default:
		return nil, fmt.Errorf("unknown image mode: %d", mode)
	}

	var partitionOptions *runtime.PartitionOptions

	if i.options.Board != constants.BoardNone && !quirks.New(i.options.Version).SupportsOverlay() {
		var b runtime.Board

		b, err := board.NewBoard(i.options.Board) //nolint:staticcheck
		if err != nil {
			return nil, err
		}

		partitionOptions = b.PartitionOptions()
	}

	if i.options.OverlayInstaller != nil {
		overlayOpts, getOptsErr := i.options.OverlayInstaller.GetOptions(ctx, i.options.ExtraOptions)
		if getOptsErr != nil {
			return nil, fmt.Errorf("failed to get overlay installer options: %w", getOptsErr)
		}

		if overlayOpts.PartitionOptions.Offset != 0 {
			partitionOptions = &runtime.PartitionOptions{
				PartitionsOffset: overlayOpts.PartitionOptions.Offset,
			}
		}
	}

	var gptOptions []gpt.Option

	if partitionOptions != nil && partitionOptions.PartitionsOffset != 0 {
		gptOptions = append(gptOptions, gpt.WithSkipLBAs(uint(partitionOptions.PartitionsOffset)))
	}

	if i.options.LegacyBIOSSupport {
		gptOptions = append(gptOptions, gpt.WithMarkPMBRBootable())
	}

	pt, err := gpt.New(gptdev, gptOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GPT: %w", err)
	}

	quirk := quirks.New(i.options.Version)

	// boot partitions
	partitions := bootlder.RequiredPartitions(quirk)

	// META partition
	partitions = append(partitions,
		partition.NewPartitionOptions(constants.MetaPartitionLabel, false, quirk),
	)

	legacyImage := mode == ModeImage && !quirks.New(i.options.Version).SkipDataPartitions()

	// compatibility when installing on Talos < 1.8
	if legacyImage || (hostTalosVersion != nil && hostTalosVersion.PrecreateStatePartition()) {
		partitions = append(partitions,
			partition.NewPartitionOptions(constants.StatePartitionLabel, false, quirk),
		)
	}

	if legacyImage {
		partitions = append(partitions,
			partition.NewPartitionOptions(constants.EphemeralPartitionLabel, false, quirk),
		)
	}

	if i.options.ImageCachePath != "" {
		partitionOptions := partition.NewPartitionOptions(constants.ImageCachePartitionLabel, false, quirk)
		partitionOptions.SourceDirectory = i.options.ImageCachePath

		partitions = append(partitions, partitionOptions)
	}

	for _, p := range partitions {
		size := p.Size

		if size == 0 {
			size = pt.LargestContiguousAllocatable()
		}

		partitionTyp := uuid.MustParse(p.PartitionType)

		_, _, err = pt.AllocatePartition(size, p.PartitionLabel, partitionTyp, p.PartitionOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate partition %s: %w", p.PartitionLabel, err)
		}

		i.options.Printf("created %s (%s) size %d bytes", p.PartitionLabel, p.PartitionType, size)
	}

	if err = pt.Write(); err != nil {
		return nil, fmt.Errorf("failed to write GPT: %w", err)
	}

	// now format all partitions
	for idx, p := range partitions {
		devName := partitioning.DevName(i.options.DiskPath, uint(idx+1))

		if err = partition.Format(devName, &p.FormatOptions, i.options.Version, i.options.Printf); err != nil {
			return nil, fmt.Errorf("failed to format partition %s: %w", devName, err)
		}
	}

	info, err := blkid.ProbePath(i.options.DiskPath, blkid.WithSkipLocking(true))
	if err != nil {
		return nil, fmt.Errorf("failed to probe blockdevice %s: %w", i.options.DiskPath, err)
	}

	if len(info.Parts) != len(partitions) {
		return nil, fmt.Errorf("expected %d partitions, got %d", len(partitions), len(info.Parts))
	}

	// this is weird, but sometimes blkid doesn't return the filesystem type for freshly formatted partitions
	for idx, p := range partitions {
		if p.FormatOptions.FileSystemType == partition.FilesystemTypeNone || p.FormatOptions.FileSystemType == partition.FilesystemTypeZeroes {
			continue
		}

		info.Parts[idx].Name = p.FormatOptions.FileSystemType
	}

	return info, nil
}

func (i *Installer) runPreflightChecks(mode Mode) error {
	if mode != ModeUpgrade {
		// pre-flight checks only apply to upgrades
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checks, err := NewPreflightChecks(ctx)
	if err != nil {
		return fmt.Errorf("error initializing pre-flight checks: %w", err)
	}

	defer checks.Close() //nolint:errcheck

	return checks.Run(ctx)
}

func overlayPresent() bool {
	_, err := os.Stat(constants.ImagerOverlayInstallerDefaultPath)

	return err == nil
}

func getBoard() string {
	cmdline := procfs.ProcCmdline()
	if cmdline == nil {
		return constants.BoardNone
	}

	board := cmdline.Get(constants.KernelParamBoard)
	if board == nil {
		return constants.BoardNone
	}

	return *board.First()
}
