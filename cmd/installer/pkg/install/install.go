// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/board"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	bootloaderoptions "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/meta"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/imager/cache"
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
	ConfigSource        string
	Disk                string
	Platform            string
	Arch                string
	Board               string
	ExtraKernelArgs     []string
	Upgrade             bool
	Force               bool
	Zero                bool
	LegacyBIOSSupport   bool
	MetaValues          MetaValues
	OverlayInstaller    overlay.Installer[overlay.ExtraOptions]
	OverlayName         string
	OverlayExtractedDir string
	ExtraOptions        overlay.ExtraOptions

	ImageCachePath string

	// Options specific for the image creation mode.
	ImageSecureboot bool
	Version         string
	BootAssets      bootloaderoptions.BootAssets
	Printf          func(string, ...any)
	MountPrefix     string
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

	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamPlatform, p.Name())

	if opts.ConfigSource != "" {
		cmdline.Append(constants.KernelParamConfig, opts.ConfigSource)
	}

	cmdline.SetAll(p.KernelArgs(opts.Arch).Strings())

	// first defaults, then extra kernel args to allow extra kernel args to override defaults
	if err := cmdline.AppendAll(kernel.DefaultArgs); err != nil {
		return err
	}

	if opts.Board != constants.BoardNone {
		// board 'rpi_4' was removed in Talos 1.5 in favor of `rpi_generic`
		if opts.Board == "rpi_4" {
			opts.Board = constants.BoardRPiGeneric
		}

		var b runtime.Board

		b, err := board.NewBoard(opts.Board)
		if err != nil {
			return err
		}

		cmdline.Append(constants.KernelParamBoard, b.Name())

		cmdline.SetAll(b.KernelArgs().Strings())
	}

	if opts.OverlayInstaller != nil {
		overlayOpts, getOptsErr := opts.OverlayInstaller.GetOptions(opts.ExtraOptions)
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
		errataArm64ZBoot()

		i.errataNetIfnames(hostTalosVersion)
	}

	if err = i.runPreflightChecks(mode); err != nil {
		return err
	}

	// prepare extensions if legacy machine.install.extensions is present
	if err = i.installExtensions(); err != nil {
		return err
	}

	// open and lock the blockdevice for the installation disk for the whole duration of the installer
	bd, err := block.NewFromPath(i.options.Disk, block.OpenForWrite())
	if err != nil {
		return fmt.Errorf("failed to open blockdevice %s: %w", i.options.Disk, err)
	}

	defer bd.Close() //nolint:errcheck

	if err = bd.Lock(true); err != nil {
		return fmt.Errorf("failed to lock blockdevice %s: %w", i.options.Disk, err)
	}

	defer bd.Unlock() //nolint:errcheck

	var bootlder bootloader.Bootloader

	// if running in install/image mode, just create new GPT partition table
	// if running in upgrade mode, verify that install disk has correct GPT partition table and expected partitions

	// probe the disk anyways
	info, err := blkid.Probe(bd.File(), blkid.WithSkipLocking(true))
	if err != nil {
		return fmt.Errorf("failed to probe blockdevice %s: %w", i.options.Disk, err)
	}

	gptdev, err := gpt.DeviceFromBlockDevice(bd)
	if err != nil {
		return fmt.Errorf("error getting GPT device: %w", err)
	}

	switch mode {
	case ModeImage:
		// on image creation, we don't care about disk contents
		bootlder = bootloader.New(i.options.ImageSecureboot, i.options.Version)
	case ModeInstall:
		if !i.options.Zero && !i.options.Force {
			// verify that the disk is either empty or has an empty GPT partition table, otherwise fail the install
			switch {
			case info.Name == "":
				// empty, ok
			case info.Name == typeGPT && len(info.Parts) == 0:
				// GPT, no partitions, ok
			default:
				return fmt.Errorf("disk %s is not empty, skipping install, detected %q", i.options.Disk, info.Name)
			}
		} else {
			// zero the disk
			if err = bd.FastWipe(); err != nil {
				return fmt.Errorf("failed to zero blockdevice %s: %w", i.options.Disk, err)
			}
		}

		// on install, automatically detect the bootloader
		bootlder = bootloader.NewAuto()
	case ModeUpgrade:
		// on upgrade, we don't touch the disk partitions, but we need to verify that the disk has the expected GPT partition table
		if info.Name != typeGPT {
			return fmt.Errorf("disk %s has an unexpected format %q", i.options.Disk, info.Name)
		}
	}

	if mode == ModeImage || mode == ModeInstall {
		// create partitions and re-probe the device
		info, err = i.createPartitions(gptdev, mode, hostTalosVersion, bootlder)
		if err != nil {
			return fmt.Errorf("failed to create partitions: %w", err)
		}

		if i.options.ImageCachePath != "" {
			cacheInstallOptions := cache.InstallOptions{
				CacheDisk: i.options.Disk,
				CachePath: i.options.ImageCachePath,
				BlkidInfo: info,
			}

			if err = cacheInstallOptions.Install(); err != nil {
				return fmt.Errorf("failed to install image cache: %w", err)
			}
		}
	}

	if mode == ModeUpgrade {
		// on upgrade, probe the bootloader
		bootlder, err = bootloader.Probe(i.options.Disk, bootloaderoptions.ProbeOptions{
			// the disk is already locked
			BlockProbeOptions: []blkid.ProbeOption{
				blkid.WithSkipLocking(true),
			},
		})
		if err != nil {
			return fmt.Errorf("failed to probe bootloader on upgrade: %w", err)
		}
	}

	// Install the bootloader.
	bootInstallResult, err := bootlder.Install(bootloaderoptions.InstallOptions{
		BootDisk:    i.options.Disk,
		Arch:        i.options.Arch,
		Cmdline:     i.cmdline.String(),
		Version:     i.options.Version,
		ImageMode:   mode.IsImage(),
		MountPrefix: i.options.MountPrefix,
		BootAssets:  i.options.BootAssets,
		Printf:      i.options.Printf,
		BlkidInfo:   info,

		ExtraInstallStep: func() error {
			if i.options.Board != constants.BoardNone {
				var b runtime.Board

				b, err = board.NewBoard(i.options.Board)
				if err != nil {
					return err
				}

				i.options.Printf("installing U-Boot for %q", b.Name())

				if err = b.Install(runtime.BoardInstallOptions{
					InstallDisk:     i.options.Disk,
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

				if err = i.options.OverlayInstaller.Install(overlay.InstallOptions[overlay.ExtraOptions]{
					InstallDisk:   i.options.Disk,
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
				metaPartitionName = partitioning.DevName(i.options.Disk, partition.PartitionIndex)

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
func (i *Installer) createPartitions(gptdev gpt.Device, mode Mode, hostTalosVersion *compatibility.TalosVersion, bootlder bootloader.Bootloader) (*blkid.Info, error) {
	var partitionOptions *runtime.PartitionOptions

	if i.options.Board != constants.BoardNone && !quirks.New(i.options.Version).SupportsOverlay() {
		var b runtime.Board

		b, err := board.NewBoard(i.options.Board)
		if err != nil {
			return nil, err
		}

		partitionOptions = b.PartitionOptions()
	}

	if i.options.OverlayInstaller != nil {
		overlayOpts, getOptsErr := i.options.OverlayInstaller.GetOptions(i.options.ExtraOptions)
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

	// boot partitions
	partitions := bootlder.RequiredPartitions()

	// META partition
	partitions = append(partitions,
		partition.NewPartitionOptions(constants.MetaPartitionLabel, false),
	)

	legacyImage := mode == ModeImage && !quirks.New(i.options.Version).SkipDataPartitions()

	// compatibility when installing on Talos < 1.8
	if legacyImage || (hostTalosVersion != nil && hostTalosVersion.PrecreateStatePartition()) {
		partitions = append(partitions,
			partition.NewPartitionOptions(constants.StatePartitionLabel, false),
		)
	}

	if legacyImage {
		partitions = append(partitions,
			partition.NewPartitionOptions(constants.EphemeralPartitionLabel, false),
		)
	}

	if i.options.ImageCachePath != "" {
		partitions = append(partitions,
			partition.NewPartitionOptions(constants.ImageCachePartitionLabel, false),
		)
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
		devName := partitioning.DevName(i.options.Disk, uint(idx+1))

		if err = partition.Format(devName, &p.FormatOptions, i.options.Printf); err != nil {
			return nil, fmt.Errorf("failed to format partition %s: %w", devName, err)
		}
	}

	info, err := blkid.ProbePath(i.options.Disk, blkid.WithSkipLocking(true))
	if err != nil {
		return nil, fmt.Errorf("failed to probe blockdevice %s: %w", i.options.Disk, err)
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
