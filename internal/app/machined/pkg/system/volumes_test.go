// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
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

func TestVolumeStatusToSelector(t *testing.T) {
	t.Parallel()

	t.Run("partition matches by partition UUID and round-trips through JSON", func(t *testing.T) {
		t.Parallel()

		const partUUID = "d2f3a1c0-0000-4000-8000-000000000001"

		vs := block.NewVolumeStatus(block.NamespaceName, "EPHEMERAL")
		vs.TypedSpec().Type = block.VolumeTypePartition
		vs.TypedSpec().PartitionUUID = partUUID

		sel, err := system.VolumeStatusToSelector(vs)
		require.NoError(t, err)

		// selectors are stored in META as a JSON array of expression strings
		data, err := json.Marshal([]cel.Expression{sel})
		require.NoError(t, err)

		var restored []cel.Expression

		require.NoError(t, json.Unmarshal(data, &restored))
		require.Len(t, restored, 1)

		env := celenv.VolumeLocator()

		match, err := restored[0].EvalBool(env, map[string]any{
			"volume": &blockpb.DiscoveredVolumeSpec{PartitionUuid: partUUID},
		})
		require.NoError(t, err)
		assert.True(t, match, "selector should match a volume with the same partition UUID")

		noMatch, err := restored[0].EvalBool(env, map[string]any{
			"volume": &blockpb.DiscoveredVolumeSpec{PartitionUuid: "00000000-0000-4000-8000-000000000002"},
		})
		require.NoError(t, err)
		assert.False(t, noMatch, "selector should not match a volume with a different partition UUID")
	})

	t.Run("partition without a UUID is an error", func(t *testing.T) {
		t.Parallel()

		vs := block.NewVolumeStatus(block.NamespaceName, "EPHEMERAL")
		vs.TypedSpec().Type = block.VolumeTypePartition

		_, err := system.VolumeStatusToSelector(vs)
		require.Error(t, err)
	})

	t.Run("non-partition volume type is unsupported", func(t *testing.T) {
		t.Parallel()

		vs := block.NewVolumeStatus(block.NamespaceName, "SOME-DIR")
		vs.TypedSpec().Type = block.VolumeTypeDirectory

		_, err := system.VolumeStatusToSelector(vs)
		require.Error(t, err)
	})
}

func TestVolumeStatusesToSelectors(t *testing.T) {
	t.Parallel()

	vs1 := block.NewVolumeStatus(block.NamespaceName, "EPHEMERAL")
	vs1.TypedSpec().Type = block.VolumeTypePartition
	vs1.TypedSpec().PartitionUUID = "d2f3a1c0-0000-4000-8000-000000000001"

	vs2 := block.NewVolumeStatus(block.NamespaceName, "STATE")
	vs2.TypedSpec().Type = block.VolumeTypePartition
	vs2.TypedSpec().PartitionUUID = "d2f3a1c0-0000-4000-8000-000000000002"

	selectors, err := system.VolumeStatusesToSelectors([]*block.VolumeStatus{vs1, vs2})
	require.NoError(t, err)
	require.Len(t, selectors, 2)

	// an error on any volume fails the whole conversion
	bad := block.NewVolumeStatus(block.NamespaceName, "VAR")
	bad.TypedSpec().Type = block.VolumeTypeDirectory

	_, err = system.VolumeStatusesToSelectors([]*block.VolumeStatus{vs1, bad})
	require.Error(t, err)
}
