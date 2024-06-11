// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// CreatePartitionResult is the result of creating a partition.
type CreatePartitionResult struct {
	PartitionIdx int
	Partition    gpt.Partition
	Size         uint64
}

// CreatePartition creates a partition on a disk.
//
//nolint:gocyclo
func CreatePartition(ctx context.Context, logger *zap.Logger, diskPath string, volumeCfg *block.VolumeConfig, hasPT bool) (CreatePartitionResult, error) {
	dev, err := blockdev.NewFromPath(diskPath, blockdev.OpenForWrite())
	if err != nil {
		return CreatePartitionResult{}, xerrors.NewTaggedf[Retryable]("error opening disk: %w", err)
	}

	defer dev.Close() //nolint:errcheck

	if err = dev.RetryLockWithTimeout(ctx, true, 10*time.Second); err != nil {
		return CreatePartitionResult{}, xerrors.NewTaggedf[Retryable]("error locking disk: %w", err)
	}

	defer dev.Unlock() //nolint:errcheck

	gptdev, err := gpt.DeviceFromBlockDevice(dev)
	if err != nil {
		return CreatePartitionResult{}, fmt.Errorf("error getting GPT device: %w", err)
	}

	var pt *gpt.Table

	if hasPT {
		pt, err = gpt.Read(gptdev)
	} else {
		pt, err = gpt.New(gptdev)
	}

	if err != nil {
		return CreatePartitionResult{}, fmt.Errorf("error initializing GPT: %w", err)
	}

	available := pt.LargestContiguousAllocatable()

	size := volumeCfg.TypedSpec().Provisioning.PartitionSpec.MinSize
	maxSize := volumeCfg.TypedSpec().Provisioning.PartitionSpec.MaxSize

	if available < size {
		// should never happen
		return CreatePartitionResult{}, fmt.Errorf("not enough space on disk")
	}

	if maxSize == 0 || maxSize >= available {
		size = available
	} else {
		size = maxSize
	}

	typeUUID, err := uuid.Parse(volumeCfg.TypedSpec().Provisioning.PartitionSpec.TypeUUID)
	if err != nil {
		return CreatePartitionResult{}, fmt.Errorf("error parsing type UUID: %w", err)
	}

	partitionIdx, partitionEntry, err := pt.AllocatePartition(size, volumeCfg.TypedSpec().Provisioning.PartitionSpec.Label, typeUUID)
	if err != nil {
		return CreatePartitionResult{}, fmt.Errorf("error allocating partition: %w", err)
	}

	if err = pt.Write(); err != nil {
		return CreatePartitionResult{}, fmt.Errorf("error writing GPT: %w", err)
	}

	// wipe the newly created partition, as it might contain old data
	partitionDevName := partitioning.DevName(diskPath, uint(partitionIdx))

	partitionDev, err := blockdev.NewFromPath(partitionDevName, blockdev.OpenForWrite())
	if err != nil {
		return CreatePartitionResult{}, xerrors.NewTaggedf[Retryable]("error opening partition: %w", err)
	}

	defer partitionDev.Close() //nolint:errcheck

	if err = partitionDev.FastWipe(); err != nil {
		return CreatePartitionResult{}, xerrors.NewTaggedf[Retryable]("error wiping partition: %w", err)
	}

	logger.Info("partition created",
		zap.String("disk", diskPath), zap.Int("partition", partitionIdx),
		zap.String("label", volumeCfg.TypedSpec().Provisioning.PartitionSpec.Label),
		zap.String("size", humanize.IBytes(size)),
	)

	return CreatePartitionResult{
		PartitionIdx: partitionIdx,
		Partition:    partitionEntry,
		Size:         size,
	}, nil
}
