// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&block.Device{},
		&block.DiscoveryRefreshRequest{},
		&block.DiscoveryRefreshStatus{},
		&block.DiscoveredVolume{},
		&block.Disk{},
		&block.MountRequest{},
		&block.MountStatus{},
		&block.SwapStatus{},
		&block.Symlink{},
		&block.SystemDisk{},
		&block.UserDiskConfigStatus{},
		&block.VolumeConfig{},
		&block.VolumeLifecycle{},
		&block.VolumeMountRequest{},
		&block.VolumeMountStatus{},
		&block.VolumeStatus{},
		&block.VolumeTrimSchedule{},
		&block.ZswapStatus{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}

func TestGetSystemDisk(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		ctx := t.Context()
		st := state.WrapCore(namespaced.NewState(inmem.Build))

		spec, err := block.GetSystemDisk(ctx, st)
		require.NoError(t, err)
		assert.Nil(t, spec)
	})

	t.Run("present", func(t *testing.T) {
		ctx := t.Context()
		st := state.WrapCore(namespaced.NewState(inmem.Build))

		createSystemDisk(ctx, t, st, "sda", "/dev/sda")

		spec, err := block.GetSystemDisk(ctx, st)
		require.NoError(t, err)
		require.NotNil(t, spec)
		assert.Equal(t, "sda", spec.DiskID)
		assert.Equal(t, "/dev/sda", spec.DevPath)
	})
}

func TestGetSystemDiskPaths(t *testing.T) {
	// a directory-backed system volume (e.g. ETCD/KUBELET/CRI as a directory on EPHEMERAL)
	// carries neither Location nor ParentLocation, so it must not contribute any disk path.
	directoryVolume := func(id string) volumeSpec {
		return volumeSpec{id: id, volumeType: block.VolumeTypeDirectory}
	}

	// a dedicated partition volume on its own disk carries the partition dev path in Location
	// and the parent disk dev path in ParentLocation.
	partitionVolume := func(id, location, parentLocation string) volumeSpec {
		return volumeSpec{id: id, volumeType: block.VolumeTypePartition, location: location, parentLocation: parentLocation}
	}

	for _, test := range []struct {
		name       string
		systemDisk string // DevPath of the system disk, "" for none
		volumes    []volumeSpec
		expected   []string
	}{
		{
			name:     "empty state",
			expected: nil,
		},
		{
			name:       "etcd/kubelet/cri/log as directories on ephemeral",
			systemDisk: "/dev/sda",
			volumes: []volumeSpec{
				partitionVolume(constants.StatePartitionLabel, "/dev/sda2", "/dev/sda"),
				partitionVolume(constants.EphemeralPartitionLabel, "/dev/sda3", "/dev/sda"),
				directoryVolume(constants.EtcdDataVolumeID),
				directoryVolume(constants.KubeletDataVolumeID),
				directoryVolume(constants.CRIContainerdVolumeID),
				directoryVolume(constants.LogVolumeID),
			},
			expected: []string{"/dev/sda"},
		},
		{
			name:       "etcd/kubelet/cri/log on dedicated disks",
			systemDisk: "/dev/sda",
			volumes: []volumeSpec{
				partitionVolume(constants.StatePartitionLabel, "/dev/sda2", "/dev/sda"),
				partitionVolume(constants.EphemeralPartitionLabel, "/dev/sda3", "/dev/sda"),
				partitionVolume(constants.EtcdDataVolumeID, "/dev/sdb1", "/dev/sdb"),
				partitionVolume(constants.KubeletDataVolumeID, "/dev/sdc1", "/dev/sdc"),
				partitionVolume(constants.CRIContainerdVolumeID, "/dev/sdd1", "/dev/sdd"),
				partitionVolume(constants.LogVolumeID, "/dev/sde1", "/dev/sde"),
			},
			expected: []string{"/dev/sda", "/dev/sdb", "/dev/sdc", "/dev/sdd", "/dev/sde"},
		},
		{
			name:       "volume without parent location falls back to location",
			systemDisk: "/dev/sda",
			volumes: []volumeSpec{
				partitionVolume(constants.EphemeralPartitionLabel, "/dev/sda3", "/dev/sda"),
				// no ParentLocation: whole-disk/external volume, Location is the disk itself.
				{id: constants.EtcdDataVolumeID, volumeType: block.VolumeTypeDisk, location: "/dev/sdb"},
			},
			expected: []string{"/dev/sda", "/dev/sdb"},
		},
		{
			name:       "dedicated volume on the system disk is not duplicated",
			systemDisk: "/dev/sda",
			volumes: []volumeSpec{
				partitionVolume(constants.EphemeralPartitionLabel, "/dev/sda3", "/dev/sda"),
				partitionVolume(constants.EtcdDataVolumeID, "/dev/sda4", "/dev/sda"),
			},
			expected: []string{"/dev/sda"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			if test.systemDisk != "" {
				createSystemDisk(ctx, t, st, "system", test.systemDisk)
			}

			for _, vol := range test.volumes {
				createVolumeStatus(ctx, t, st, vol)
			}

			paths, err := block.GetSystemDiskPaths(ctx, st)
			require.NoError(t, err)
			assert.ElementsMatch(t, test.expected, paths)
		})
	}
}

// volumeSpec describes a VolumeStatus to seed into the test state.
type volumeSpec struct {
	id             string
	volumeType     block.VolumeType
	location       string
	parentLocation string
}

func createSystemDisk(ctx context.Context, t *testing.T, st state.State, diskID, devPath string) {
	t.Helper()

	systemDisk := block.NewSystemDisk(block.NamespaceName, block.SystemDiskID)
	systemDisk.TypedSpec().DiskID = diskID
	systemDisk.TypedSpec().DevPath = devPath

	require.NoError(t, st.Create(ctx, systemDisk))
}

func createVolumeStatus(ctx context.Context, t *testing.T, st state.State, spec volumeSpec) {
	t.Helper()

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, spec.id)
	volumeStatus.TypedSpec().Type = spec.volumeType
	volumeStatus.TypedSpec().Location = spec.location
	volumeStatus.TypedSpec().ParentLocation = spec.parentLocation

	require.NoError(t, st.Create(ctx, volumeStatus))
}
