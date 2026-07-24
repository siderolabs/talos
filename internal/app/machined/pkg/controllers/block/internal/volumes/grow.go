// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/gen/xerrors"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Grow grows a volume.
func Grow(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	if volumeContext.Cfg.TypedSpec().Type == block.VolumeTypePartition && volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.Grow {
		// record the disk size the partition is grown for, so that live growth can pick up further disk size changes
		volumeContext.recordObservedDiskSize()

		if err := growPartition(ctx, logger, volumeContext); err != nil {
			return err
		}
	}

	volumeContext.Status.Phase = block.VolumePhaseProvisioned

	return nil
}

// GrowLive grows a ready (and possibly mounted) volume if the underlying disk has grown.
//
// The new size of the volume is propagated to the volume status, and the filesystem
// is grown by the mount controller.
//
// Live growth is only supported for unencrypted volumes on top of raw disks.
func GrowLive(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	if !volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.Grow {
		return nil
	}

	if volumeContext.Cfg.TypedSpec().Encryption.Provider != block.EncryptionProviderNone {
		return nil
	}

	switch volumeContext.Cfg.TypedSpec().Type { //nolint:exhaustive
	case block.VolumeTypeDisk:
		growLiveDisk(logger, volumeContext)
	case block.VolumeTypePartition:
		return growLivePartition(ctx, logger, volumeContext)
	}

	return nil
}

// growLiveDisk propagates the new size of the disk to the disk volume.
func growLiveDisk(logger *zap.Logger, volumeContext ManagerContext) {
	disk := volumeContext.FindDisk(volumeContext.Status.Location)
	if disk == nil || disk.Size <= volumeContext.Status.Size {
		return
	}

	logger.Info("disk volume grown",
		zap.String("disk", volumeContext.Status.Location),
		zap.String("size", fmt.Sprintf("%d -> %d", volumeContext.Status.Size, disk.Size)),
	)

	volumeContext.Status.SetSize(disk.Size)
}

// growLivePartition grows the partition volume if the parent disk size has changed since it was last grown.
func growLivePartition(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	disk := volumeContext.FindDisk(volumeContext.Status.ParentLocation)
	if disk == nil {
		return nil
	}

	observedSize, ok := volumeContext.ObservedDiskSizes[volumeContext.Cfg.Metadata().ID()]
	if !ok {
		// first time seeing the volume ready (e.g. after a restart), the partition was grown when provisioned
		volumeContext.recordObservedDiskSize()

		return nil
	}

	if observedSize == disk.Size {
		return nil
	}

	prevSize := volumeContext.Status.Size

	if err := growPartition(ctx, logger, volumeContext); err != nil {
		return err
	}

	volumeContext.recordObservedDiskSize()

	if volumeContext.Status.Size != prevSize {
		logger.Info("partition volume grown",
			zap.String("disk", volumeContext.Status.ParentLocation),
			zap.Int("partition", volumeContext.Status.PartitionIndex),
			zap.String("size", fmt.Sprintf("%d -> %d", prevSize, volumeContext.Status.Size)),
		)
	}

	return nil
}

// growPartition grows the partition to fill the available space on the disk (up to the max size).
//
// On success, the new size of the partition is set in the volume status.
//
//nolint:gocyclo
func growPartition(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	if volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize > 0 && volumeContext.Status.Size >= volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize {
		// nowhere to grow
		return nil
	}

	dev, err := blockdev.NewFromPath(volumeContext.Status.ParentLocation, blockdev.OpenForWrite())
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error opening disk: %w", err)
	}

	defer dev.Close() //nolint:errcheck

	if err = dev.RetryLockWithTimeout(ctx, true, 10*time.Second); err != nil {
		return xerrors.NewTaggedf[Retryable]("error locking disk: %w", err)
	}

	defer dev.Unlock() //nolint:errcheck

	gptdev, err := gpt.DeviceFromBlockDevice(dev)
	if err != nil {
		return fmt.Errorf("error getting GPT device: %w", err)
	}

	pt, err := gpt.Read(gptdev)
	if err != nil {
		return fmt.Errorf("error initializing GPT: %w", err)
	}

	availableGrowth, err := pt.AvailablePartitionGrowth(volumeContext.Status.PartitionIndex - 1)
	if err != nil {
		return fmt.Errorf("error getting available partition growth: %w", err)
	}

	if availableGrowth <= 1024*1024 { // don't grow by less than 1MiB
		return nil
	}

	if volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize > 0 && availableGrowth > volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize-volumeContext.Status.Size {
		availableGrowth = volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize - volumeContext.Status.Size
	}

	logger.Debug("growing partition", zap.String("disk", volumeContext.Status.ParentLocation), zap.Int("partition", volumeContext.Status.PartitionIndex), zap.Uint64("size", availableGrowth))

	if err = pt.GrowPartition(volumeContext.Status.PartitionIndex-1, availableGrowth); err != nil {
		return fmt.Errorf("error growing partition: %w", err)
	}

	if err = pt.Write(); err != nil {
		return fmt.Errorf("error writing GPT: %w", err)
	}

	volumeContext.Status.SetSize(volumeContext.Status.Size + availableGrowth)

	return nil
}
