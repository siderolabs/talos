// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mdd

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// levelRAID1 is the only MD level Add restores, as reported by mdadm.
const levelRAID1 = "raid1"

// ArrayInfo is the observed state of the array a disk is added to.
type ArrayInfo struct {
	// Name is the metadata-stamped array name, e.g. talos:boot.
	Name string
	// Level is the observed MD level, e.g. raid1.
	Level string
	// Degraded is the number of RAID devices the array is missing.
	Degraded int
	// SyncAction is the sync action in progress (e.g. resync, recover, check), empty or "idle" if none.
	SyncAction string
	// Members are the /dev paths of every device attached to the array, failed and spare ones included.
	Members []string
	// InSyncMembers are the /dev paths of the members which carry the array data.
	InSyncMembers []string
}

// ValidateAdd checks that the disk named member may be added to the array to restore its redundancy.
//
// Add only restores the redundancy the array already declares, onto a disk which holds nothing:
// it never creates a spare, never grows the array and never overwrites a disk with recognizable
// content. The returned error is a gRPC status describing the first unmet precondition.
func ValidateAdd(ctx context.Context, st state.State, array ArrayInfo, member string) error {
	if err := validateArrayForAdd(array, member); err != nil {
		return err
	}

	if err := validateArrayUnmanaged(ctx, st, array); err != nil {
		return err
	}

	disk, err := wholeDisk(ctx, st, member)
	if err != nil {
		return err
	}

	if disk.TypedSpec().Readonly {
		return status.Errorf(codes.FailedPrecondition, "disk %q is read-only", member)
	}

	if disk.TypedSpec().CDROM {
		return status.Errorf(codes.FailedPrecondition, "disk %q is a CD-ROM", member)
	}

	// md, device-mapper, loop and other virtual block devices have no physical slot to rebuild onto
	if disk.TypedSpec().BusPath == "/virtual" {
		return status.Errorf(codes.InvalidArgument, "disk %q is a virtual block device", member)
	}

	if err := validateDiskUnused(ctx, st, member); err != nil {
		return err
	}

	if err := validateDiskBlank(ctx, st, member); err != nil {
		return err
	}

	return validateDiskFitsArray(ctx, st, array, disk)
}

func validateArrayForAdd(array ArrayInfo, member string) error {
	switch {
	case array.Level != levelRAID1:
		return status.Errorf(codes.FailedPrecondition, "only raid1 arrays are supported, array level is %q", array.Level)
	case slices.Contains(array.Members, filepath.Join("/dev", member)):
		return status.Errorf(codes.AlreadyExists, "disk %q is already attached to the array", member)
	case array.Degraded == 0:
		return status.Error(codes.FailedPrecondition, "array is not degraded: the disk would only become a spare")
	case array.SyncAction != "" && array.SyncAction != string(md.SyncActionIdle):
		return status.Errorf(codes.FailedPrecondition, "array has a %q sync action in progress: wait for it to finish", array.SyncAction)
	case len(array.InSyncMembers) == 0:
		return status.Error(codes.FailedPrecondition, "array has no in-sync member to rebuild from")
	}

	return nil
}

// validateArrayUnmanaged refuses arrays created from a RAIDArrayConfig: the MD reconciler adds the
// disks matching its selector to them, and the two must not race to fill the same slot.
func validateArrayUnmanaged(ctx context.Context, st state.State, array ArrayInfo) error {
	specs, err := safe.StateListAll[*storage.MDArraySpec](ctx, st)
	if err != nil {
		return err
	}

	for spec := range specs.All() {
		// the reconciler stamps the array with the name the md-name-<name> by-id link is made of
		if filepath.Base(md.DevicePath(spec.Metadata().ID())) == "md-name-"+array.Name {
			return status.Errorf(codes.FailedPrecondition,
				"array is managed by RAIDArrayConfig %q: make its volume selector match the new disk instead", spec.Metadata().ID())
		}
	}

	return nil
}

func wholeDisk(ctx context.Context, st state.State, name string) (*block.Disk, error) {
	device, err := safe.StateGetByID[*block.Device](ctx, st, name)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "blockdevice %q not found", name)
		}

		return nil, err
	}

	if device.TypedSpec().Type != block.DeviceTypeDisk {
		return nil, status.Errorf(codes.InvalidArgument, "blockdevice %q is not a whole disk", name)
	}

	disk, err := safe.StateGetByID[*block.Disk](ctx, st, name)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.NotFound, "disk %q not found", name)
		}

		return nil, err
	}

	return disk, nil
}

func validateDiskUnused(ctx context.Context, st state.State, member string) error {
	systemDisk, err := safe.StateGetByID[*block.SystemDisk](ctx, st, block.SystemDiskID)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	if systemDisk != nil && systemDisk.TypedSpec().DiskID == member {
		return status.Errorf(codes.FailedPrecondition, "disk %q is the system disk", member)
	}

	disks, err := safe.StateListAll[*block.Disk](ctx, st)
	if err != nil {
		return err
	}

	for disk := range disks.All() {
		if slices.Contains(disk.TypedSpec().SecondaryDisks, member) {
			return status.Errorf(codes.FailedPrecondition, "disk %q is in use by disk %q", member, disk.Metadata().ID())
		}
	}

	return validateDiskNotVolume(ctx, st, member)
}

func validateDiskNotVolume(ctx context.Context, st state.State, member string) error {
	volumeStatuses, err := safe.StateListAll[*block.VolumeStatus](ctx, st)
	if err != nil {
		return err
	}

	for volumeStatus := range volumeStatuses.All() {
		for _, location := range []string{
			volumeStatus.TypedSpec().Location,
			volumeStatus.TypedSpec().MountLocation,
			volumeStatus.TypedSpec().ParentLocation,
		} {
			if location != "" && filepath.Base(location) == member {
				return status.Errorf(codes.FailedPrecondition, "disk %q is in use by volume %q", member, volumeStatus.Metadata().ID())
			}
		}
	}

	return nil
}

func validateDiskBlank(ctx context.Context, st state.State, member string) error {
	volumes, err := safe.StateListAll[*block.DiscoveredVolume](ctx, st)
	if err != nil {
		return err
	}

	probed := false

	for volume := range volumes.All() {
		switch {
		case volume.Metadata().ID() == member:
			probed = true

			if name := volume.TypedSpec().Name; name != "" {
				return notBlank(member, fmt.Sprintf("%s signature found", name))
			}
		case volume.TypedSpec().Parent == member:
			return notBlank(member, "it has partitions")
		}
	}

	if !probed {
		return status.Errorf(codes.FailedPrecondition, "disk %q has not been probed yet", member)
	}

	return nil
}

func notBlank(member, reason string) error {
	return status.Errorf(codes.FailedPrecondition, "disk %q is not blank (%s): wipe it first with `talosctl wipe disk %s`", member, reason, member)
}

func validateDiskFitsArray(ctx context.Context, st state.State, array ArrayInfo, disk *block.Disk) error {
	member := disk.Metadata().ID()

	for _, path := range array.InSyncMembers {
		existing, err := wholeDisk(ctx, st, filepath.Base(path))
		if err != nil {
			if status.Code(err) == codes.InvalidArgument {
				return status.Errorf(codes.FailedPrecondition, "array member %q is not a whole disk: only arrays of whole disks are supported", path)
			}

			return err
		}

		if disk.TypedSpec().Size < existing.TypedSpec().Size {
			return status.Errorf(codes.FailedPrecondition, "disk %q (%s) is smaller than array member %q (%s)",
				member, disk.TypedSpec().PrettySize, existing.Metadata().ID(), existing.TypedSpec().PrettySize)
		}

		// mdadm accepts the disk either way, but the partition table written through the array
		// uses the members' sector size: on a disk with another sector size the firmware no
		// longer finds it, so the rebuilt member would silently stop being bootable.
		if disk.TypedSpec().SectorSize != existing.TypedSpec().SectorSize {
			return status.Errorf(codes.FailedPrecondition, "disk %q has a logical sector size of %d bytes, array member %q has %d bytes",
				member, disk.TypedSpec().SectorSize, existing.Metadata().ID(), existing.TypedSpec().SectorSize)
		}
	}

	return nil
}
