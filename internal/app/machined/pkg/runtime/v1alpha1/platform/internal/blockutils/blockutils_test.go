// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockutils_test

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/blockutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/xfs"
)

func TestVolumeMatch(t *testing.T) {
	t.Parallel()

	expr, err := blockutils.VolumeMatch([]string{constants.MetalConfigISOLabel})
	require.NoError(t, err)

	assert.Equal(t, `(volume.label in ["metal-iso"] || volume.partition_label in ["metal-iso"]) && volume.name != ""`, expr.String())
}

func TestReadFromVolumeConflict(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	// another invocation with the same requester name already owns the volume configuration
	existing := block.NewVolumeConfig(block.NamespaceName, "platform/"+constants.MetalConfigISOLabel+"/"+blockutils.RequesterConfig)
	require.NoError(t, st.Create(ctx, existing))

	err := blockutils.ReadFromVolume(ctx, st, blockutils.RequesterConfig, []string{constants.MetalConfigISOLabel},
		func(xfs.Root, *block.VolumeStatus) error {
			t.Fatal("callback should not be called")

			return nil
		},
	)
	require.Error(t, err)
	assert.True(t, state.IsConflictError(err), "expected conflict error, got %v", err)

	// the volume configuration owned by the other invocation must be left untouched
	vc, err := safe.StateGetResource(ctx, st, existing)
	require.NoError(t, err)
	assert.Equal(t, resource.PhaseRunning, vc.Metadata().Phase())
}

func TestReadFromVolumeMissing(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	volumeID := "platform/" + constants.MetalConfigISOLabel + "/" + blockutils.RequesterNetwork

	// distinct requester names produce distinct volume configurations
	other := block.NewVolumeConfig(block.NamespaceName, "platform/"+constants.MetalConfigISOLabel+"/"+blockutils.RequesterConfig)
	require.NoError(t, st.Create(ctx, other))

	// emulate the volume manager: report the volume as missing once the configuration is created
	go func() {
		vc, err := safe.StateWatchFor[*block.VolumeConfig](ctx, st, block.NewVolumeConfig(block.NamespaceName, volumeID).Metadata(), state.WithEventTypes(state.Created))
		if err != nil {
			return
		}

		vs := block.NewVolumeStatus(block.NamespaceName, vc.Metadata().ID())
		vs.TypedSpec().Phase = block.VolumePhaseMissing

		st.Create(ctx, vs) //nolint:errcheck
	}()

	err := blockutils.ReadFromVolume(ctx, st, blockutils.RequesterNetwork, []string{constants.MetalConfigISOLabel},
		func(xfs.Root, *block.VolumeStatus) error {
			t.Fatal("callback should not be called")

			return nil
		},
	)
	require.ErrorIs(t, err, fs.ErrNotExist)

	// the volume configuration created by this invocation is destroyed on return
	_, err = st.Get(ctx, block.NewVolumeConfig(block.NamespaceName, volumeID).Metadata())
	assert.True(t, state.IsNotFoundError(err), "expected not found error, got %v", err)

	// the volume configuration of the other requester is not affected
	vc, err := safe.StateGetResource(ctx, st, other)
	require.NoError(t, err)
	assert.Equal(t, resource.PhaseRunning, vc.Metadata().Phase())
}
