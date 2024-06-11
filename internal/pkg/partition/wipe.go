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
	VolumeStatus *blockres.VolumeStatus
}

// GetLabel implements runtime.PartitionTarget.
func (v *VolumeWipeTarget) GetLabel() string {
	return v.VolumeStatus.Metadata().ID()
}

// Wipe implements runtime.PartitionTarget.
func (v *VolumeWipeTarget) Wipe(ctx context.Context, log func(string, ...any)) error {
	parentDevName := v.VolumeStatus.TypedSpec().ParentLocation

	if parentDevName == "" {
		parentDevName = v.VolumeStatus.TypedSpec().Location
	}

	parentBd, err := block.NewFromPath(parentDevName)
	if err != nil {
		return fmt.Errorf("error opening block device %q: %w", parentDevName, err)
	}

	defer parentBd.Close() //nolint:errcheck

	if err = parentBd.RetryLockWithTimeout(ctx, true, time.Minute); err != nil {
		return fmt.Errorf("error locking block device %q: %w", parentDevName, err)
	}

	defer parentBd.Unlock() //nolint:errcheck

	bd, err := block.NewFromPath(v.VolumeStatus.TypedSpec().Location, block.OpenForWrite())
	if err != nil {
		return fmt.Errorf("error opening block device %q: %w", v.VolumeStatus.TypedSpec().Location, err)
	}

	defer bd.Close() //nolint:errcheck

	log("wiping the volume %q (%s)", v.GetLabel(), v.VolumeStatus.TypedSpec().Location)

	return bd.FastWipe()
}
