// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package runtime

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestSystemVolumeStatuses(t *testing.T) {
	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	sysVol := block.NewVolumeStatus(block.NamespaceName, "EPHEMERAL")
	sysVol.Metadata().Labels().Set(block.SystemVolumeLabel, "")
	require.NoError(t, st.Create(ctx, sysVol))

	userVol := block.NewVolumeStatus(block.NamespaceName, "my-user-volume")
	userVol.Metadata().Labels().Set(block.UserVolumeLabel, "")
	require.NoError(t, st.Create(ctx, userVol))

	t.Run("valid system volume", func(t *testing.T) {
		result, err := resolveSystemVolumeStatuses(ctx, st, []string{"EPHEMERAL"})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "EPHEMERAL", result[0].Metadata().ID())
	})

	t.Run("unknown volume", func(t *testing.T) {
		_, err := resolveSystemVolumeStatuses(ctx, st, []string{"NONEXISTENT"})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, grpcstatus.Code(err))
	})

	t.Run("non-system volume", func(t *testing.T) {
		_, err := resolveSystemVolumeStatuses(ctx, st, []string{"my-user-volume"})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, grpcstatus.Code(err))
	})
}

func TestAssertVolumesNotMounted(t *testing.T) {
	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	mountStatus := block.NewVolumeMountStatus(block.NamespaceName, "EPHEMERAL-machined")
	mountStatus.TypedSpec().VolumeID = "EPHEMERAL"
	require.NoError(t, st.Create(ctx, mountStatus))

	t.Run("mounted volume rejected", func(t *testing.T) {
		err := assertVolumesNotMounted(ctx, st, []string{"EPHEMERAL"})
		require.Error(t, err)
		assert.Equal(t, codes.FailedPrecondition, grpcstatus.Code(err))
		assert.Contains(t, err.Error(), "retry with --on-reboot")
	})

	t.Run("unmounted volume allowed", func(t *testing.T) {
		require.NoError(t, assertVolumesNotMounted(ctx, st, []string{"STATE"}))
	})
}
