// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package partition

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"

	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeWipeTarget is a target for wiping a volume.
type VolumeWipeTarget struct {
	label string

	parentDevName, devName string

	partitionIndex int // partitionIndex is 1-based, but decrement before using
}

// VolumeWipeTargetFromVolumeStatus creates a new VolumeWipeTarget from a VolumeStatus.
func VolumeWipeTargetFromVolumeStatus(vs *blockres.VolumeStatus) *VolumeWipeTarget {
	parentDevName := vs.TypedSpec().ParentLocation

	if parentDevName == "" {
		parentDevName = vs.TypedSpec().Location
	}

	return &VolumeWipeTarget{
		label:          vs.Metadata().ID(),
		parentDevName:  parentDevName,
		devName:        vs.TypedSpec().Location,
		partitionIndex: vs.TypedSpec().PartitionIndex,
	}
}

// VolumeWipeTargetFromDiscoveredVolume creates a new VolumeWipeTarget from a DiscoveredVolume.
func VolumeWipeTargetFromDiscoveredVolume(dv *blockres.DiscoveredVolume) *VolumeWipeTarget {
	parentDevName := dv.TypedSpec().ParentDevPath

	if parentDevName == "" {
		parentDevName = dv.TypedSpec().DevPath
	}

	return &VolumeWipeTarget{
		label:          dv.TypedSpec().PartitionLabel,
		parentDevName:  parentDevName,
		devName:        dv.TypedSpec().DevPath,
		partitionIndex: int(dv.TypedSpec().PartitionIndex),
	}
}

// GetLabel implements runtime.PartitionTarget.
func (v *VolumeWipeTarget) GetLabel() string {
	return v.label
}

// String implements runtime.PartitionTarget.
func (v *VolumeWipeTarget) String() string {
	return fmt.Sprintf("%s:%s", v.label, v.devName)
}

// Wipe implements runtime.PartitionTarget.
// Asides from wiping the device, Wipe() also drops the partition.
func (v *VolumeWipeTarget) Wipe(ctx context.Context, log func(string, ...any)) error {
	parentBd, err := block.NewFromPath(v.parentDevName, block.OpenForWrite())
	if err != nil {
		return fmt.Errorf("error opening block device %q: %w", v.parentDevName, err)
	}

	defer parentBd.Close() //nolint:errcheck

	if err = parentBd.RetryLockWithTimeout(ctx, true, time.Minute); err != nil {
		return fmt.Errorf("error locking block device %q: %w", v.parentDevName, err)
	}

	defer parentBd.Unlock() //nolint:errcheck

	if err := v.wipeWithParentLocked(log); err != nil {
		return fmt.Errorf("error wiping device %q: %w", v.devName, err)
	}

	if parentBd == nil || v.partitionIndex == 0 {
		return fmt.Errorf("missing parent block device or partition index")
	}

	if err := v.dropWithParentLocked(parentBd, log); err != nil {
		return fmt.Errorf("error dropping partition: %w", err)
	}

	return nil
}

func (v *VolumeWipeTarget) wipeWithParentLocked(log func(string, ...any)) error {
	bd, err := block.NewFromPath(v.devName, block.OpenForWrite())
	if err != nil {
		return fmt.Errorf("error opening block device %q: %w", v.devName, err)
	}

	defer bd.Close() //nolint:errcheck

	log("wiping volume %q (%s)", v.GetLabel(), v.devName)

	return WipeWithSignatures(bd, v.devName, log)
}

func (v *VolumeWipeTarget) dropWithParentLocked(parentBd *block.Device, log func(string, ...any)) error {
	log("dropping partition %d from device %q", v.partitionIndex, v.parentDevName)

	gptdev, err := gpt.DeviceFromBlockDevice(parentBd)
	if err != nil {
		return fmt.Errorf("failed to get GPT device: %w", err)
	}

	pt, err := gpt.Read(gptdev)
	if err != nil {
		return fmt.Errorf("failed to read GPT table: %w", err)
	}

	if err = pt.DeletePartition(v.partitionIndex - 1); err != nil {
		return fmt.Errorf("failed to delete partition: %w", err)
	}

	if err = pt.Write(); err != nil {
		return fmt.Errorf("failed to write GPT table: %w", err)
	}

	return nil
}
