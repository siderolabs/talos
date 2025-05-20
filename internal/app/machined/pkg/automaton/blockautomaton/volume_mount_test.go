// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockautomaton_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/owned"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestVolumeMounter(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	st := state.WrapCore(namespaced.NewState(inmem.Build))
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	mountedCh := make(chan struct{})

	volumeMounter := blockautomaton.NewVolumeMounter("requester", "volumeID", func(ctx context.Context, rw controller.ReaderWriter, l *zap.Logger, vms *block.VolumeMountStatus) error {
		select {
		case <-mountedCh:
			// already closed
			return nil
		default:
			close(mountedCh)

			return errors.New("mount status callback")
		}
	})

	const mountID = "requester-volumeID"

	adapter := owned.New(st, "automaton")

	// 1st run, should create the volume mount request
	require.NoError(t, volumeMounter.Run(ctx, adapter, logger))

	rtestutils.AssertResource(ctx, t, st, mountID, func(vmr *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal("requester", vmr.TypedSpec().Requester)
		asrt.Equal("volumeID", vmr.TypedSpec().VolumeID)
	})

	require.NoError(t, st.AddFinalizer(ctx, block.NewVolumeMountRequest(block.NamespaceName, mountID).Metadata(), "test"))

	// no-op run, as the volume mount status doesn't exist
	require.NoError(t, volumeMounter.Run(ctx, adapter, logger))

	vms := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	require.NoError(t, st.Create(ctx, vms))

	// 2nd run, should put a finalizer on the volume mount status and call the callback 1st time
	err := volumeMounter.Run(ctx, adapter, logger)

	select {
	case <-mountedCh:
	case <-ctx.Done():
		t.Fatal("timed out waiting for mount status callback")
	}

	require.ErrorContains(t, err, "mount status callback")

	// should put a finalizer on the volume mount status
	rtestutils.AssertResource(ctx, t, st, mountID, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Has("requester"))
	})

	// 3rd run, now the mount callback should be called again, return nil,
	// and volume mount status finalizer should be removed
	require.NoError(t, volumeMounter.Run(ctx, adapter, logger))

	// should remove a finalizer on the volume mount status
	rtestutils.AssertResource(ctx, t, st, mountID, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.False(vms.Metadata().Finalizers().Has("requester"))
	})

	// the mount request now should be torn down by the automaton
	rtestutils.AssertResource(ctx, t, st, mountID, func(vmr *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, vmr.Metadata().Phase())
	})

	// remove our finalizer from the mount request
	require.NoError(t, st.RemoveFinalizer(ctx, block.NewVolumeMountRequest(block.NamespaceName, mountID).Metadata(), "test"))

	// 4th run, now the mount request should be destroyed
	require.NoError(t, volumeMounter.Run(ctx, adapter, logger))

	rtestutils.AssertNoResource[*block.VolumeMountRequest](ctx, t, st, mountID)

	// destroy the volume mount status
	require.NoError(t, st.Destroy(ctx, vms.Metadata()))

	var finished bool

	// 5th run, now the automaton should have finished
	require.NoError(t, volumeMounter.Run(ctx, adapter, logger, automaton.WithAfterFunc(func() error {
		finished = true

		return nil
	})))

	assert.True(t, finished)
}
