// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/swap"
	"go.uber.org/zap"

	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
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

			volumeContext.Status.Filesystem, _ = block.FilesystemTypeString(info.Name) //nolint:errcheck
		} else {
			volumeContext.Status.Filesystem = block.FilesystemTypeNone
		}

		volumeContext.Status.Phase = block.VolumePhaseReady

		return nil
	case info.Name == "":
		// no filesystem, format
	case info.Name == "swap":
		// swap volume, format always
	case info.Name == volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type.String():
		// filesystem already exists and matches the requested type
		if volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.Grow {
			// if the partition is set to grow, we need to grow the filesystem
			if err = GrowFilesystem(logger, volumeContext); err != nil {
				return fmt.Errorf("error growing filesystem: %w", err)
			}
		}

		volumeContext.Status.Phase = block.VolumePhaseReady
		volumeContext.Status.Filesystem = volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type

		return nil
	default:
		// mismatch
		return fmt.Errorf("filesystem type mismatch: %s != %s", info.Name, volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}

	logger.Info("formatting filesystem",
		zap.String("device", volumeContext.Status.MountLocation),
		zap.Stringer("filesystem", volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type),
	)

	switch volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type { //nolint:exhaustive
	case block.FilesystemTypeXFS:
		var makefsOptions []makefs.Option

		// xfs doesn't support by default filesystems < 300 MiB
		if volumeContext.Status.Size <= 300*1024*1024 {
			makefsOptions = append(makefsOptions, makefs.WithUnsupportedFSOption(true))
		}

		if volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label != "" {
			makefsOptions = append(makefsOptions, makefs.WithLabel(volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label))
		}

		makefsOptions = append(makefsOptions, makefs.WithConfigFile(quirks.New("").XFSMkfsConfig()))

		if err = makefs.XFS(volumeContext.Status.MountLocation, makefsOptions...); err != nil {
			return fmt.Errorf("error formatting XFS: %w", err)
		}
	case block.FilesystemTypeEXT4:
		var makefsOptions []makefs.Option

		if volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label != "" {
			makefsOptions = append(makefsOptions, makefs.WithLabel(volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label))
		}

		if err = makefs.Ext4(volumeContext.Status.MountLocation, makefsOptions...); err != nil {
			return fmt.Errorf("error formatting ext4: %w", err)
		}
	case block.FilesystemTypeSwap:
		if err = swap.Format(volumeContext.Status.MountLocation, swap.FormatOptions{
			Label: volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Label,
			UUID:  uuid.New(),
		}); err != nil {
			return fmt.Errorf("error formatting swap: %w", err)
		}
	default:
		return fmt.Errorf("unsupported filesystem type: %s", volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}

	volumeContext.Status.Phase = block.VolumePhaseReady
	volumeContext.Status.Filesystem = volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type

	return nil
}

// GrowFilesystem grows the filesystem on the block device.
func GrowFilesystem(logger *zap.Logger, volumeContext ManagerContext) error {
	switch volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type { //nolint:exhaustive
	case block.FilesystemTypeXFS:
		// XFS requires partition to be mounted to grow
		tmpDir, err := os.MkdirTemp("", "talos-growfs-")
		if err != nil {
			return fmt.Errorf("error creating temporary directory: %w", err)
		}

		defer os.Remove(tmpDir) //nolint:errcheck

		mountpoint := mountv2.NewPoint(volumeContext.Status.MountLocation, tmpDir, volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type.String())

		unmounter, err := mountpoint.Mount(mountv2.WithMountPrinter(logger.Sugar().Infof))
		if err != nil {
			return fmt.Errorf("error mounting partition: %w", err)
		}

		defer unmounter() //nolint:errcheck

		logger.Info("growing XFS filesystem", zap.String("device", volumeContext.Status.MountLocation))

		if err = makefs.XFSGrow(tmpDir); err != nil {
			return fmt.Errorf("error growing XFS: %w", err)
		}

		return nil
	case block.FilesystemTypeEXT4:
		logger.Info("growing ext4 filesystem", zap.String("device", volumeContext.Status.MountLocation))

		if err := makefs.Ext4Resize(volumeContext.Status.MountLocation); err != nil {
			return fmt.Errorf("error growing ext4: %w", err)
		}

		return nil
	case block.FilesystemTypeSwap:
		// swap is always reformatted, so we don't need to grow it
		return nil
	default:
		return fmt.Errorf("unsupported filesystem type to grow: %s", volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}
}
