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
//
//nolint:gocyclo
func Grow(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	if !(volumeContext.Cfg.TypedSpec().Type == block.VolumeTypePartition && volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.Grow) {
		// nothing to do
		volumeContext.Status.Phase = block.VolumePhaseProvisioned

		return nil
	}

	if volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize > 0 && volumeContext.Status.Size >= volumeContext.Cfg.TypedSpec().Provisioning.PartitionSpec.MaxSize {
		// nowhere to grow
		volumeContext.Status.Phase = block.VolumePhaseProvisioned

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
		volumeContext.Status.Phase = block.VolumePhaseProvisioned

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

	volumeContext.Status.Phase = block.VolumePhaseProvisioned
	volumeContext.Status.SetSize(volumeContext.Status.Size + availableGrowth)

	return nil
}
