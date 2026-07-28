// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// volumeSpec describes a VolumeStatus to seed into the test state.
type volumeSpec struct {
	id string
	// location is the block device backing the volume; empty for volumes which don't have one of
	// their own (directories, overlays, ...).
	location string
	// parentID is the top-level VolumeStatusSpec.ParentID, set for overlay volumes.
	parentID string
	// mountParentID is VolumeStatusSpec.MountSpec.ParentID, set for mounted volumes.
	mountParentID string
}

func createVolumeStatus(ctx context.Context, t *testing.T, st state.State, spec volumeSpec) {
	t.Helper()

	volumeStatus := block.NewVolumeStatus(block.NamespaceName, spec.id)
	volumeStatus.TypedSpec().Location = spec.location
	volumeStatus.TypedSpec().ParentID = spec.parentID
	volumeStatus.TypedSpec().MountSpec.ParentID = spec.mountParentID

	if spec.location == "" {
		volumeStatus.TypedSpec().Type = block.VolumeTypeDirectory
	} else {
		volumeStatus.TypedSpec().Type = block.VolumeTypePartition
	}

	require.NoError(t, st.Create(ctx, volumeStatus))
}

// realLayout mirrors the default (non-promoted) system volume layout: the promotable volumes are
// directories nested under EPHEMERAL, some of them via the intermediate /var/lib directory volume.
func realLayout() []volumeSpec {
	return []volumeSpec{
		{id: constants.EphemeralPartitionLabel, location: "/dev/vda6"},
		{id: "/var/lib", mountParentID: constants.EphemeralPartitionLabel},
		{id: constants.EtcdDataVolumeID, mountParentID: "/var/lib"},
		{id: constants.CRIContainerdVolumeID, mountParentID: "/var/lib"},
		{id: constants.KubeletDataVolumeID, mountParentID: "/var/lib"},
		{id: constants.LogVolumeID, mountParentID: constants.EphemeralPartitionLabel},
	}
}

func TestFindBackingVolume(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		volumes []volumeSpec
		// volumeID is the volume to resolve the backing volume for.
		volumeID string

		expected    string
		expectedErr string
	}{
		{
			name:     "nested under an intermediate directory",
			volumes:  realLayout(),
			volumeID: constants.KubeletDataVolumeID,
			expected: constants.EphemeralPartitionLabel,
		},
		{
			name:     "direct child of the backing volume",
			volumes:  realLayout(),
			volumeID: constants.LogVolumeID,
			expected: constants.EphemeralPartitionLabel,
		},
		{
			name:     "intermediate directory volume itself",
			volumes:  realLayout(),
			volumeID: "/var/lib",
			expected: constants.EphemeralPartitionLabel,
		},
		{
			name: "overlay volume using the top-level parent ID",
			volumes: []volumeSpec{
				{id: constants.EphemeralPartitionLabel, location: "/dev/vda6"},
				{id: "/etc/cni", parentID: constants.EphemeralPartitionLabel},
			},
			volumeID: "/etc/cni",
			expected: constants.EphemeralPartitionLabel,
		},
		{
			name: "top-level parent ID takes precedence over the mount parent",
			volumes: []volumeSpec{
				{id: constants.EphemeralPartitionLabel, location: "/dev/vda6"},
				{id: constants.StatePartitionLabel, location: "/dev/vda5"},
				{id: "/opt", parentID: constants.EphemeralPartitionLabel, mountParentID: constants.StatePartitionLabel},
			},
			volumeID: "/opt",
			expected: constants.EphemeralPartitionLabel,
		},
		{
			name: "no parent at all",
			volumes: []volumeSpec{
				{id: "tmp"},
			},
			volumeID:    "tmp",
			expectedErr: `volume "tmp" is not located and doesn't reside on another volume`,
		},
		{
			name: "parent volume status is missing",
			volumes: []volumeSpec{
				{id: constants.EtcdDataVolumeID, mountParentID: "/var/lib"},
			},
			volumeID:    constants.EtcdDataVolumeID,
			expectedErr: `failed to get parent volume status "/var/lib" of volume "ETCD"`,
		},
		{
			name: "self-referencing parent",
			volumes: []volumeSpec{
				{id: "/var/lib", mountParentID: "/var/lib"},
			},
			volumeID:    "/var/lib",
			expectedErr: `cycle detected in the parent chain of volume "/var/lib" at "/var/lib"`,
		},
		{
			name: "cycle between two volumes",
			volumes: []volumeSpec{
				{id: "/var/lib", mountParentID: "/var/lib/kubelet"},
				{id: "/var/lib/kubelet", mountParentID: "/var/lib"},
			},
			volumeID:    "/var/lib",
			expectedErr: `cycle detected in the parent chain of volume "/var/lib" at "/var/lib"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			for _, spec := range test.volumes {
				createVolumeStatus(ctx, t, st, spec)
			}

			backingVolume, err := system.FindBackingVolume(ctx, st, test.volumeID)

			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, backingVolume)
		})
	}
}
