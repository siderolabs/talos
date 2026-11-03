// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mdd_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/mdd"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

const gib = 1024 * 1024 * 1024

func degradedMirror() mdd.ArrayInfo {
	return mdd.ArrayInfo{
		Level:         "raid1",
		Degraded:      1,
		SyncAction:    "idle",
		Members:       []string{"/dev/sda"},
		InSyncMembers: []string{"/dev/sda"},
	}
}

func createDisk(ctx context.Context, t *testing.T, st state.State, id string, size uint64, sectorSize uint) *block.Disk {
	t.Helper()

	device := block.NewDevice(block.NamespaceName, id)
	device.TypedSpec().Type = block.DeviceTypeDisk
	require.NoError(t, st.Create(ctx, device))

	disk := block.NewDisk(block.NamespaceName, id)
	disk.TypedSpec().DevPath = "/dev/" + id
	disk.TypedSpec().SetSize(size)
	disk.TypedSpec().SectorSize = sectorSize
	require.NoError(t, st.Create(ctx, disk))

	return disk
}

func createDiscoveredVolume(ctx context.Context, t *testing.T, st state.State, id, parent, name string) {
	t.Helper()

	volume := block.NewDiscoveredVolume(block.NamespaceName, id)
	volume.TypedSpec().Parent = parent
	volume.TypedSpec().Name = name
	require.NoError(t, st.Create(ctx, volume))
}

// newState returns a state with a degraded mirror's surviving member sda (raid member) and a blank
// 10 GiB disk sdb with matching sector size.
func newState(ctx context.Context, t *testing.T) state.State {
	t.Helper()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	createDisk(ctx, t, st, "sda", 10*gib, 512)
	createDiscoveredVolume(ctx, t, st, "sda", "", "linux_raid_member")

	createDisk(ctx, t, st, "sdb", 10*gib, 512)
	createDiscoveredVolume(ctx, t, st, "sdb", "", "")

	return st
}

func TestValidateAddAcceptsBlankDisk(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newState(ctx, t)

	require.NoError(t, mdd.ValidateAdd(ctx, st, degradedMirror(), "sdb"))
}

func TestValidateAddArrayPreconditions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		mutate func(*mdd.ArrayInfo)
		code   codes.Code
		msg    string
	}{
		{
			name:   "not raid1",
			mutate: func(a *mdd.ArrayInfo) { a.Level = "raid5" },
			code:   codes.FailedPrecondition,
			msg:    "only raid1",
		},
		{
			name:   "not degraded",
			mutate: func(a *mdd.ArrayInfo) { a.Degraded = 0 },
			code:   codes.FailedPrecondition,
			msg:    "not degraded",
		},
		{
			name:   "rebuilding",
			mutate: func(a *mdd.ArrayInfo) { a.SyncAction = "recover" },
			code:   codes.FailedPrecondition,
			msg:    `"recover" sync action in progress`,
		},
		{
			name:   "checking",
			mutate: func(a *mdd.ArrayInfo) { a.SyncAction = "check" },
			code:   codes.FailedPrecondition,
			msg:    `"check" sync action in progress`,
		},
		{
			name:   "no in-sync member",
			mutate: func(a *mdd.ArrayInfo) { a.InSyncMembers = nil },
			code:   codes.FailedPrecondition,
			msg:    "no in-sync member",
		},
		{
			name:   "already attached as a spare",
			mutate: func(a *mdd.ArrayInfo) { a.Members = append(a.Members, "/dev/sdb") },
			code:   codes.AlreadyExists,
			msg:    "already attached",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := newState(ctx, t)

			array := degradedMirror()
			test.mutate(&array)

			err := mdd.ValidateAdd(ctx, st, array, "sdb")
			assert.Equal(t, test.code, status.Code(err), "error: %v", err)
			assert.ErrorContains(t, err, test.msg)
		})
	}
}

func TestValidateAddDiskPreconditions(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		member string
		setup  func(ctx context.Context, t *testing.T, st state.State)
		code   codes.Code
		msg    string
	}{
		{
			name:   "unknown disk",
			member: "sdz",
			code:   codes.NotFound,
			msg:    "not found",
		},
		{
			name:   "partition",
			member: "sdb1",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				partition := block.NewDevice(block.NamespaceName, "sdb1")
				partition.TypedSpec().Type = block.DeviceTypePartition
				partition.TypedSpec().Parent = "sdb"
				require.NoError(t, st.Create(ctx, partition))
			},
			code: codes.InvalidArgument,
			msg:  "not a whole disk",
		},
		{
			name:   "virtual device",
			member: "md126",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				md := createDisk(ctx, t, st, "md126", 10*gib, 512)
				md.TypedSpec().BusPath = "/virtual"
				require.NoError(t, st.Update(ctx, md))
				createDiscoveredVolume(ctx, t, st, "md126", "", "")
			},
			code: codes.InvalidArgument,
			msg:  "virtual block device",
		},
		{
			name:   "system disk",
			member: "sdb",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				systemDisk := block.NewSystemDisk(block.NamespaceName, block.SystemDiskID)
				systemDisk.TypedSpec().DiskID = "sdb"
				require.NoError(t, st.Create(ctx, systemDisk))
			},
			code: codes.FailedPrecondition,
			msg:  "system disk",
		},
		{
			name:   "member of another array",
			member: "sdb",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				md := createDisk(ctx, t, st, "md127", 10*gib, 512)
				md.TypedSpec().SecondaryDisks = []string{"sdb"}
				require.NoError(t, st.Update(ctx, md))
			},
			code: codes.FailedPrecondition,
			msg:  `in use by disk "md127"`,
		},
		{
			name:   "used by a volume",
			member: "sdb",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				volumeStatus := block.NewVolumeStatus(block.NamespaceName, "u-data")
				volumeStatus.TypedSpec().Location = "/dev/sdb"
				require.NoError(t, st.Create(ctx, volumeStatus))
			},
			code: codes.FailedPrecondition,
			msg:  `in use by volume "u-data"`,
		},
		{
			name:   "filesystem signature",
			member: "sdc",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				createDisk(ctx, t, st, "sdc", 10*gib, 512)
				createDiscoveredVolume(ctx, t, st, "sdc", "", "xfs")
			},
			code: codes.FailedPrecondition,
			msg:  "not blank (xfs signature found)",
		},
		{
			// a former member whose superblock alone was zeroed still carries the array's partition table
			name:   "partition table",
			member: "sdc",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				createDisk(ctx, t, st, "sdc", 10*gib, 512)
				createDiscoveredVolume(ctx, t, st, "sdc", "", "gpt")
			},
			code: codes.FailedPrecondition,
			msg:  "not blank (gpt signature found)",
		},
		{
			name:   "partitions",
			member: "sdc",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				createDisk(ctx, t, st, "sdc", 10*gib, 512)
				createDiscoveredVolume(ctx, t, st, "sdc", "", "")
				createDiscoveredVolume(ctx, t, st, "sdc1", "sdc", "")
			},
			code: codes.FailedPrecondition,
			msg:  "it has partitions",
		},
		{
			name:   "not probed yet",
			member: "sdc",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				createDisk(ctx, t, st, "sdc", 10*gib, 512)
			},
			code: codes.FailedPrecondition,
			msg:  "not been probed",
		},
		{
			name:   "too small",
			member: "sdc",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				createDisk(ctx, t, st, "sdc", 5*gib, 512)
				createDiscoveredVolume(ctx, t, st, "sdc", "", "")
			},
			code: codes.FailedPrecondition,
			msg:  "smaller than array member",
		},
		{
			name:   "sector size mismatch",
			member: "sdc",
			setup: func(ctx context.Context, t *testing.T, st state.State) {
				createDisk(ctx, t, st, "sdc", 10*gib, 4096)
				createDiscoveredVolume(ctx, t, st, "sdc", "", "")
			},
			code: codes.FailedPrecondition,
			msg:  "logical sector size of 4096 bytes",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := newState(ctx, t)

			if test.setup != nil {
				test.setup(ctx, t, st)
			}

			err := mdd.ValidateAdd(ctx, st, degradedMirror(), test.member)
			assert.Equal(t, test.code, status.Code(err), "error: %v", err)
			assert.ErrorContains(t, err, test.msg)
		})
	}
}

func TestValidateAddRejectsManagedArrays(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newState(ctx, t)

	require.NoError(t, st.Create(ctx, storage.NewMDArraySpec(storage.NamespaceName, "data")))

	array := degradedMirror()
	array.Name = "talos:boot"

	// another array's config does not matter
	require.NoError(t, mdd.ValidateAdd(ctx, st, array, "sdb"))

	require.NoError(t, st.Create(ctx, storage.NewMDArraySpec(storage.NamespaceName, "boot")))

	err := mdd.ValidateAdd(ctx, st, array, "sdb")
	assert.Equal(t, codes.FailedPrecondition, status.Code(err), "error: %v", err)
	assert.ErrorContains(t, err, `managed by RAIDArrayConfig "boot"`)
}

func TestValidateAddRejectsPartitionMemberArrays(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st := newState(ctx, t)

	partition := block.NewDevice(block.NamespaceName, "sda3")
	partition.TypedSpec().Type = block.DeviceTypePartition
	partition.TypedSpec().Parent = "sda"
	require.NoError(t, st.Create(ctx, partition))

	array := degradedMirror()
	array.Members = []string{"/dev/sda3"}
	array.InSyncMembers = []string{"/dev/sda3"}

	err := mdd.ValidateAdd(ctx, st, array, "sdb")
	assert.Equal(t, codes.FailedPrecondition, status.Code(err), "error: %v", err)
	assert.ErrorContains(t, err, "only arrays of whole disks")
}
