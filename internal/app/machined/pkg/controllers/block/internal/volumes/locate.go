// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/partitioning"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LocateAndProvision locates and provisions a volume.
//
// It verifies that the volume is either already created, and ready to be used,
// or provisions it.
//
//nolint:gocyclo,cyclop
func LocateAndProvision(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	volumeType := volumeContext.Cfg.TypedSpec().Type

	if volumeType == block.VolumeTypeTmpfs {
		// tmpfs volumes are always ready
		volumeContext.Status.Phase = block.VolumePhaseReady

		return nil
	}

	// below for partition/disk volumes:
	if value.IsZero(volumeContext.Cfg.TypedSpec().Locator) {
		return fmt.Errorf("volume locator is not set")
	}

	// attempt to discover the volume
	for _, dv := range volumeContext.DiscoveredVolumes {
		matches, err := volumeContext.Cfg.TypedSpec().Locator.Match.EvalBool(celenv.VolumeLocator(), map[string]any{
			"volume": dv,
		})
		if err != nil {
			return fmt.Errorf("error evaluating volume locator: %w", err)
		}

		if matches {
			volumeContext.Status.Phase = block.VolumePhaseLocated
			volumeContext.Status.Location = dv.DevPath
			volumeContext.Status.PartitionIndex = int(dv.PartitionIndex)
			volumeContext.Status.ParentLocation = dv.ParentDevPath

			volumeContext.Status.UUID = dv.Uuid
			volumeContext.Status.PartitionUUID = dv.PartitionUuid
			volumeContext.Status.SetSize(dv.Size)

			return nil
		}
	}

	if !volumeContext.DevicesReady {
		// volume wasn't located and devices are not ready yet, so we need to wait
		volumeContext.Status.Phase = block.VolumePhaseWaiting

		return nil
	}

	// if we got here, the volume is missing, so it needs to be provisioned
	if value.IsZero(volumeContext.Cfg.TypedSpec().Provisioning) {
		// the volume can't be provisioned, because the provisioning instructions are missing
		volumeContext.Status.Phase = block.VolumePhaseMissing

		return nil
	}

	if !volumeContext.PreviousWaveProvisioned {
		// previous wave is not provisioned yet
		volumeContext.Status.Phase = block.VolumePhaseWaiting

		return nil
	}

	// locate the disk(s) for the volume
	var matchedDisks []string

	for _, diskCtx := range volumeContext.Disks {
		if diskCtx.Disk.Readonly {
			// skip readonly disks, they can't be provisioned either way
			continue
		}

		matches, err := volumeContext.Cfg.TypedSpec().Provisioning.DiskSelector.Match.EvalBool(celenv.DiskLocator(), map[string]any{
			"disk":        diskCtx.Disk,
			"system_disk": diskCtx.SystemDisk,
		})
		if err != nil {
			return fmt.Errorf("error evaluating disk locator: %w", err)
		}

		if matches {
			matchedDisks = append(matchedDisks, diskCtx.Disk.DevPath)
		}
	}

	if len(matchedDisks) == 0 {
		return fmt.Errorf("no disks matched for volume")
	}

	logger.Debug("matched disks", zap.Strings("disks", matchedDisks))

	// analyze each disk, until we find the one which is the best fit
	var (
		pickedDisk      string
		diskCheckResult CheckDiskResult
	)

	for _, matchedDisk := range matchedDisks {
		diskCheckResult = CheckDiskForProvisioning(logger, matchedDisk, volumeContext.Cfg)
		if diskCheckResult.CanProvision {
			pickedDisk = matchedDisk

			break
		}
	}

	if pickedDisk == "" {
		return xerrors.NewTaggedf[Retryable]("no disks matched for volume")
	}

	logger.Debug("picked disk", zap.String("disk", pickedDisk))

	switch volumeType { //nolint:exhaustive
	case block.VolumeTypeDisk:
		// the disk got matched, so we are done here
		volumeContext.Status.Phase = block.VolumePhaseProvisioned
		volumeContext.Status.Location = pickedDisk
		volumeContext.Status.ParentLocation = ""
		volumeContext.Status.SetSize(diskCheckResult.DiskSize)
	case block.VolumeTypePartition:
		// we need to create a partition on the disk
		partitionCreateResult, err := CreatePartition(ctx, logger, pickedDisk, volumeContext.Cfg, diskCheckResult.HasGPT)
		if err != nil {
			return fmt.Errorf("error creating partition: %w", err)
		}

		volumeContext.Status.Phase = block.VolumePhaseProvisioned
		volumeContext.Status.Location = partitioning.DevName(pickedDisk, uint(partitionCreateResult.PartitionIdx))
		volumeContext.Status.PartitionIndex = partitionCreateResult.PartitionIdx
		volumeContext.Status.ParentLocation = pickedDisk
		volumeContext.Status.PartitionUUID = partitionCreateResult.Partition.PartGUID.String()
		volumeContext.Status.SetSize(partitionCreateResult.Size)
	default:
		panic("unexpected volume type")
	}

	return nil
}
