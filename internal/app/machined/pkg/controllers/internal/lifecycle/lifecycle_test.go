// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lifecycle_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/lifecycle"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const owner = "test.Controller"

func TestContainerLifecycle(t *testing.T) {
	testBarrier(t, func() *containers.ContainerLifecycle {
		return containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID)
	})
}

func TestVirtualMachineLifecycle(t *testing.T) {
	testBarrier(t, func() *hypervisor.VirtualMachineLifecycle {
		return hypervisor.NewVirtualMachineLifecycle(hypervisor.NamespaceName, hypervisor.VirtualMachineLifecycleID)
	})
}

func testBarrier[T any, PT lifecycle.Barrier[T]](t *testing.T, newBarrier func() PT) {
	id := newBarrier().Metadata().ID()

	t.Run("read", func(t *testing.T) {
		st := state.WrapCore(inmem.NewState())

		barrier, err := lifecycle.Read[T, PT](t.Context(), st, id)
		require.NoError(t, err)
		assert.Nil(t, barrier)

		require.NoError(t, st.Create(t.Context(), newBarrier()))

		barrier, err = lifecycle.Read[T, PT](t.Context(), st, id)
		require.NoError(t, err)
		require.NotNil(t, barrier)
		assert.Equal(t, id, barrier.Metadata().ID())
	})

	t.Run("shutting down", func(t *testing.T) {
		tearingDown := newBarrier()
		tearingDown.Metadata().SetPhase(resource.PhaseTearingDown)

		assert.False(t, lifecycle.ShuttingDown(newBarrier()))
		assert.True(t, lifecycle.ShuttingDown(tearingDown))
		assert.True(t, lifecycle.ShuttingDown[T, PT](nil))
	})

	t.Run("reconcile", func(t *testing.T) {
		ctx := t.Context()
		logger := zaptest.NewLogger(t)
		st := state.WrapCore(inmem.NewState())

		read := func() PT {
			barrier, err := safe.StateGetByID[PT](ctx, st, id)
			require.NoError(t, err)

			return barrier
		}

		// Nothing to hold once the barrier is gone.
		require.NoError(t, lifecycle.Reconcile[T, PT](ctx, st, logger, nil, owner, true))

		require.NoError(t, st.Create(ctx, newBarrier()))

		// Held while running, whether or not there is anything to wind down.
		for _, releasable := range []bool{true, true, false} {
			require.NoError(t, lifecycle.Reconcile(ctx, st, logger, read(), owner, releasable))
			assert.True(t, read().Metadata().Finalizers().Has(owner))
		}

		_, err := st.Teardown(ctx, newBarrier().Metadata())
		require.NoError(t, err)

		// Kept while something is still winding down.
		require.NoError(t, lifecycle.Reconcile(ctx, st, logger, read(), owner, false))
		assert.True(t, read().Metadata().Finalizers().Has(owner))

		// Released once nothing is, and never taken again on the way down.
		for range 2 {
			require.NoError(t, lifecycle.Reconcile(ctx, st, logger, read(), owner, true))
			assert.False(t, read().Metadata().Finalizers().Has(owner))
		}
	})
}
