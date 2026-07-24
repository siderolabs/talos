// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/swap"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/makefs"
)

// Format establishes a filesystem on a block device.
//
//nolint:gocyclo,cyclop
func Format(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	// lock either the parent device or the device itself
	devPath := volumeContext.Status.ParentLocation
	if devPath == "" {
		devPath = volumeContext.Status.Location
	}

	dev, err := blockdev.NewFromPath(devPath)
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error opening disk: %w", err)
	}

	defer dev.Close() //nolint:errcheck

	if err = dev.RetryLockWithTimeout(ctx, true, 10*time.Second); err != nil {
		return xerrors.NewTaggedf[Retryable]("error locking disk: %w", err)
	}

	defer dev.Unlock() //nolint:errcheck

	info, err := blkid.ProbePath(volumeContext.Status.MountLocation, blkid.WithSkipLocking(true))
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error probing disk: %w", err)
	}

	switch {
	case volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type == block.FilesystemTypeNone:
		volumeContext.Status.Filesystem, _ = block.FilesystemTypeString(info.Name) //nolint:errcheck

		// this is mountable
		if volumeContext.Cfg.TypedSpec().Mount.TargetPath != "" {
			switch info.Name {
			case "":
				return fmt.Errorf("filesystem not found on %s", volumeContext.Status.MountLocation)
			case "luks":
				// this volume is actually encrypted, but we got here without encryption config, move phase back
				volumeContext.Status.Phase = block.VolumePhaseProvisioned

				return fmt.Errorf("volume is encrypted, but no encryption config provided")
			}
		}

		volumeContext.Status.Phase = block.VolumePhaseReady

		return nil
	case info.Name == "":
		// no filesystem, format
	case info.Name == "swap":
		// swap volume, format always
	case info.Name == volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type.String():
		// filesystem already exists and matches the requested type
		//
		// if the volume is set to grow, the filesystem is grown by the mount controller
		volumeContext.Status.Phase = block.VolumePhaseReady
		volumeContext.Status.Filesystem = volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type

		return nil
	default:
		// mismatch
		return fmt.Errorf("filesystem type mismatch: %s != %s", info.Name, volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}

	logger.Info(
		"formatting filesystem",
		zap.String("device", volumeContext.Status.MountLocation),
		zap.Stringer("filesystem", volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type),
	)

	makefsOptions := []makefs.Option{makefs.WithPrintf(logger.Sugar().Debugf)}

	switch volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type { //nolint:exhaustive
	case block.FilesystemTypeXFS:
		// xfs doesn't support by default filesystems < 300 MiB
		if volumeContext.Status.Size <= 300*1024*1024 {
			makefsOptions = append(makefsOptions, makefs.WithUnsupportedFSOption(true))
		}

		if volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label != "" {
			makefsOptions = append(makefsOptions, makefs.WithLabel(volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label))
		}

		makefsOptions = append(makefsOptions, makefs.WithConfigFile(quirks.New("").XFSMkfsConfig()))

		// mkfs.xfs only scales the allocation group count (and the journal size) to the CPU count on
		// non-rotational devices, so the minimum allocation group size is only relevant there.
		if disk := volumeContext.FindDisk(cmp.Or(volumeContext.Status.ParentLocation, volumeContext.Status.Location)); disk != nil && !disk.Rotational {
			makefsOptions = append(makefsOptions,
				makefs.WithDeviceSize(volumeContext.Status.Size),
				makefs.WithMinAllocationGroupSize(volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.MinAllocationGroupSize),
			)
		}

		if err = makefs.XFS(ctx, volumeContext.Status.MountLocation, makefsOptions...); err != nil {
			return xerrors.NewTaggedf[Retryable]("error formatting XFS: %w", err)
		}
	case block.FilesystemTypeEXT4:
		if volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label != "" {
			makefsOptions = append(makefsOptions, makefs.WithLabel(volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label))
		}

		if err = makefs.Ext4(ctx, volumeContext.Status.MountLocation, makefsOptions...); err != nil {
			return xerrors.NewTaggedf[Retryable]("error formatting ext4: %w", err)
		}
	case block.FilesystemTypeBtrfs:
		if volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label != "" {
			makefsOptions = append(makefsOptions, makefs.WithLabel(volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label))
		}

		if err = makefs.BTRFS(ctx, volumeContext.Status.MountLocation, makefsOptions...); err != nil {
			return xerrors.NewTaggedf[Retryable]("error formatting btrfs: %w", err)
		}
	case block.FilesystemTypeSwap:
		if err = swap.Format(volumeContext.Status.MountLocation, swap.FormatOptions{
			Label: volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label,
			UUID:  uuid.New(),
		}); err != nil {
			return xerrors.NewTaggedf[Retryable]("error formatting swap: %w", err)
		}
	default:
		return fmt.Errorf("unsupported filesystem type: %s", volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}

	volumeContext.Status.Phase = block.VolumePhaseReady
	volumeContext.Status.Filesystem = volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type

	return nil
}
