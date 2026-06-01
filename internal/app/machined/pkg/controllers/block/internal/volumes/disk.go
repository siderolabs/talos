// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// DiskRejectedReason is the reason why a disk cannot be used for provisioning.
type DiskRejectedReason int

// Possible reasons why a disk cannot be used for provisioning.
const (
	GeneralError DiskRejectedReason = iota
	NotEnoughSpace
	WrongFormat
)

// CheckDiskResult is the result of checking a disk for provisioning.
type CheckDiskResult struct {
	// CanProvision indicates if the disk can be used for provisioning.
	CanProvision bool
	// HasGPT indicates if the disk has a GPT partition table.
	HasGPT bool
	// DiskSize is the size of the disk.
	DiskSize uint64
	// RejectedReason is the reason why the disk cannot be used for provisioning (if CanProvision is false).
	RejectedReason DiskRejectedReason
}

// CheckDiskForProvisioning checks if the disk can be used for provisioning for the given volume configuration.
func CheckDiskForProvisioning(logger *zap.Logger, diskPath string, volumeCfg *block.VolumeConfig) CheckDiskResult {
	info, err := blkid.ProbePath(diskPath)
	if err != nil {
		logger.Error("error probing disk", zap.String("disk", diskPath), zap.Error(err))

		return CheckDiskResult{}
	}

	switch volumeCfg.TypedSpec().Type { //nolint:exhaustive
	case block.VolumeTypeDisk:
		return CheckDiskResult{
			CanProvision:   info.Name == "",
			DiskSize:       info.Size,
			RejectedReason: WrongFormat,
		}
	case block.VolumeTypePartition:
		if info.Name == "" {
			// if the disk is not partitioned, it can be used for partitioning, but we need to check the size
			overhead := uint64(info.SectorSize) * 67 // GPT + MBR

			return CheckDiskResult{
				CanProvision:   info.Size >= volumeCfg.TypedSpec().Provisioning.PartitionSpec.MinSize+overhead,
				DiskSize:       info.Size,
				RejectedReason: NotEnoughSpace,
			}
		}

		if info.Name != "gpt" {
			// not empty, and not gpt => can't be used for partitioning
			return CheckDiskResult{
				RejectedReason: WrongFormat,
			}
		}
	default:
		panic("unexpected volume type")
	}

	// the rest for partition type volumes with existing GPT partition table
	// find the amount of space available
	dev, err := blockdev.NewFromPath(diskPath)
	if err != nil {
		logger.Error("error opening disk", zap.String("disk", diskPath), zap.Error(err))

		return CheckDiskResult{}
	}

	defer dev.Close() //nolint:errcheck

	if err = dev.TryLock(false); err != nil {
		logger.Error("error locking disk", zap.String("disk", diskPath), zap.Error(err))

		return CheckDiskResult{}
	}

	defer dev.Unlock() //nolint:errcheck

	gptdev, err := gpt.DeviceFromBlockDevice(dev)
	if err != nil {
		logger.Error("error getting GPT device", zap.String("disk", diskPath), zap.Error(err))

		return CheckDiskResult{}
	}

	pt, err := gpt.Read(gptdev)
	if err != nil {
		logger.Error("error reading GPT", zap.String("disk", diskPath), zap.Error(err))

		return CheckDiskResult{}
	}

	available := pt.LargestContiguousAllocatable()

	logger.Debug("checking disk for provisioning", zap.String("disk", diskPath), zap.Uint64("available", available))

	return CheckDiskResult{
		CanProvision:   available >= volumeCfg.TypedSpec().Provisioning.PartitionSpec.MinSize,
		HasGPT:         true,
		DiskSize:       info.Size,
		RejectedReason: NotEnoughSpace,
	}
}
