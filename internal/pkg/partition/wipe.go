// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package partition

import (
	"context"
	"fmt"
	"time"

	"github.com/siderolabs/go-blockdevice/v2/block"

	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// VolumeWipeTarget is a target for wiping a volume.
type VolumeWipeTarget struct {
	label string

	parentDevName, devName string
}

// VolumeWipeTargetFromVolumeStatus creates a new VolumeWipeTarget from a VolumeStatus.
func VolumeWipeTargetFromVolumeStatus(vs *blockres.VolumeStatus) *VolumeWipeTarget {
	parentDevName := vs.TypedSpec().ParentLocation

	if parentDevName == "" {
		parentDevName = vs.TypedSpec().Location
	}

	return &VolumeWipeTarget{
		label:         vs.Metadata().ID(),
		parentDevName: parentDevName,
		devName:       vs.TypedSpec().Location,
	}
}

// VolumeWipeTargetFromDiscoveredVolume creates a new VolumeWipeTarget from a DiscoveredVolume.
func VolumeWipeTargetFromDiscoveredVolume(dv *blockres.DiscoveredVolume) *VolumeWipeTarget {
	parentDevName := dv.TypedSpec().ParentDevPath

	if parentDevName == "" {
		parentDevName = dv.TypedSpec().DevPath
	}

	return &VolumeWipeTarget{
		label:         dv.TypedSpec().PartitionLabel,
		parentDevName: parentDevName,
		devName:       dv.TypedSpec().DevPath,
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
func (v *VolumeWipeTarget) Wipe(ctx context.Context, log func(string, ...any)) error {
	parentBd, err := block.NewFromPath(v.parentDevName)
	if err != nil {
		return fmt.Errorf("error opening block device %q: %w", v.parentDevName, err)
	}

	defer parentBd.Close() //nolint:errcheck

	if err = parentBd.RetryLockWithTimeout(ctx, true, time.Minute); err != nil {
		return fmt.Errorf("error locking block device %q: %w", v.parentDevName, err)
	}

	defer parentBd.Unlock() //nolint:errcheck

	bd, err := block.NewFromPath(v.devName, block.OpenForWrite())
	if err != nil {
		return fmt.Errorf("error opening block device %q: %w", v.devName, err)
	}

	defer bd.Close() //nolint:errcheck

	log("wiping the volume %q (%s)", v.GetLabel(), v.devName)

	return bd.FastWipe()
}
