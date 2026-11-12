// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

const testController = "etcd.MembershipController"

func newRequestState(t *testing.T, phase etcd.MembershipPhase) state.State {
	t.Helper()

	st := state.WrapCore(inmem.NewState())

	status := etcd.NewMembershipStatus()
	status.TypedSpec().Phase = phase

	require.NoError(t, st.Create(t.Context(), status))

	return st
}

func TestRequestLeaveWithdrawn(t *testing.T) {
	t.Parallel()

	st := newRequestState(t, etcd.MembershipPhaseMember)

	// nothing handles the request, so the leave times out
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := etcdcli.RequestLeave(ctx, st)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// the failed request should not be acted upon later
	_, err = safe.StateGetByID[*etcd.LeaveRequest](t.Context(), st, etcd.LeaveRequestID)
	assert.True(t, state.IsNotFoundError(err), "unexpected error: %v", err)
}

func TestRequestLeaveCommitted(t *testing.T) {
	t.Parallel()

	st := newRequestState(t, etcd.MembershipPhaseMember)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	// the controller removes the member from the cluster, which is the point of no return
	go func() {
		request, err := st.WatchFor(ctx, etcd.NewLeaveRequest().Metadata(), state.WithEventTypes(state.Created))
		if err != nil {
			return
		}

		st.AddFinalizer(ctx, request.Metadata(), testController) //nolint:errcheck
	}()

	err := etcdcli.RequestLeave(ctx, st)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, "still in effect")

	request, err := safe.StateGetByID[*etcd.LeaveRequest](t.Context(), st, etcd.LeaveRequestID)
	require.NoError(t, err)
	assert.True(t, request.Metadata().Finalizers().Has(testController))
}

func TestRequestBootstrapReplace(t *testing.T) {
	t.Parallel()

	st := newRequestState(t, etcd.MembershipPhaseJoining)

	require.NoError(t, etcdcli.RequestBootstrap(t.Context(), st, etcd.BootstrapRequestSpec{}))

	// the request is not relevant anymore, and the controller holds it
	require.NoError(t, st.AddFinalizer(t.Context(), etcd.NewBootstrapRequest().Metadata(), testController))

	// the controller releases the request once it is replaced
	go func() {
		request, err := st.WatchFor(t.Context(), etcd.NewBootstrapRequest().Metadata(), state.WithPhases(resource.PhaseTearingDown))
		if err != nil {
			return
		}

		st.RemoveFinalizer(t.Context(), request.Metadata(), testController) //nolint:errcheck
	}()

	require.NoError(t, etcdcli.RequestBootstrap(t.Context(), st, etcd.BootstrapRequestSpec{RecoverFromSnapshot: true}))

	request, err := safe.StateGetByID[*etcd.BootstrapRequest](t.Context(), st, etcd.BootstrapRequestID)
	require.NoError(t, err)

	assert.True(t, request.TypedSpec().RecoverFromSnapshot)
	assert.True(t, request.Metadata().Finalizers().Empty())
	assert.Equal(t, resource.PhaseRunning, request.Metadata().Phase())
}
