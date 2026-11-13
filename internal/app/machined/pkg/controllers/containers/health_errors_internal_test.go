// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/siderolabs/gen/optional"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

var errInjected = errors.New("injected")

// faultyRuntime is a controller runtime over a state, failing the operations it is told to.
//
// Only what the controllers under test call is implemented: anything else panics on the nil
// embedded Runtime.
type faultyRuntime struct {
	controller.Runtime

	state  state.State
	events chan controller.ReconcileEvent

	failGet     resource.Type
	failList    resource.Type
	failModify  bool
	failCleanup bool
}

func newFaultyRuntime() *faultyRuntime {
	return &faultyRuntime{
		state:  state.WrapCore(inmem.NewState()),
		events: make(chan controller.ReconcileEvent, 1),
	}
}

func (r *faultyRuntime) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if ptr.Type() == r.failGet {
		return nil, errInjected
	}

	return r.state.Get(ctx, ptr, opts...)
}

func (r *faultyRuntime) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if kind.Type() == r.failList {
		return resource.List{}, errInjected
	}

	return r.state.List(ctx, kind, opts...)
}

func (r *faultyRuntime) Modify(context.Context, resource.Resource, func(resource.Resource) error, ...controller.ModifyOption) error {
	if r.failModify {
		return errInjected
	}

	return nil
}

func (r *faultyRuntime) StartTrackingOutputs() {}

func (r *faultyRuntime) CleanupOutputs(context.Context, ...resource.Kind) error {
	if r.failCleanup {
		return errInjected
	}

	return nil
}

func (r *faultyRuntime) EventCh() <-chan controller.ReconcileEvent { return r.events }

func (r *faultyRuntime) ResetRestartBackoff() {}

const faultyTestContainer = "app"

var faultyTestHealthCheck = containers.ContainerHealthCheckSpec{
	Kind:             string(config.ContainerHealthCheckKindTCP),
	Port:             80,
	Interval:         10 * time.Millisecond,
	Timeout:          10 * time.Millisecond,
	FailureThreshold: 1,
}

// seedRunningContainer creates a health-checked container with a running instance, so that every
// read of a reconcile is reached.
func seedRunningContainer(t *testing.T, st state.State) {
	t.Helper()

	ctx := t.Context()
	id := containers.InstanceID(faultyTestContainer, 0)

	spec := containers.NewContainerSpec(containers.NamespaceName, faultyTestContainer)
	spec.TypedSpec().HealthCheck = faultyTestHealthCheck
	require.NoError(t, st.Create(ctx, spec))

	instanceSpec := containers.NewContainerInstanceSpec(containers.NamespaceName, id)
	instanceSpec.TypedSpec().ContainerID = faultyTestContainer
	require.NoError(t, st.Create(ctx, instanceSpec))

	instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, id)
	instanceStatus.TypedSpec().ContainerID = faultyTestContainer
	instanceStatus.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
	instanceStatus.TypedSpec().PID = 42
	instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, faultyTestContainer)
	require.NoError(t, st.Create(ctx, instanceStatus))

	// An outcome already published, so that the probe is decided as soon as it starts and its
	// outcome is written in the same pass.
	healthStatus := containers.NewContainerHealthStatus(containers.NamespaceName, id)
	healthStatus.TypedSpec().Healthy = true
	require.NoError(t, st.Create(ctx, healthStatus))
}

// passingProbe is a probe provider whose checks always pass.
func passingProbe(ProbeTarget) (Probe, io.Closer, error) {
	return func(context.Context) error { return nil }, nopCloser{}, nil
}

// TestHealthReconcileErrors covers the reads and writes of a reconcile failing: the error is
// returned, so that the controller is restarted, rather than read as nothing to probe.
func TestHealthReconcileErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		fault   func(*faultyRuntime)
		wantErr string
	}{
		{
			name:    "list instance statuses",
			fault:   func(r *faultyRuntime) { r.failList = containers.ContainerInstanceStatusType },
			wantErr: "failed to list instance statuses",
		},
		{
			name:    "get instance spec",
			fault:   func(r *faultyRuntime) { r.failGet = containers.ContainerInstanceSpecType },
			wantErr: `failed to get instance spec "app-0"`,
		},
		{
			name:    "get container spec",
			fault:   func(r *faultyRuntime) { r.failGet = containers.ContainerSpecType },
			wantErr: `failed to get container spec "app"`,
		},
		{
			name:    "get previous outcome",
			fault:   func(r *faultyRuntime) { r.failGet = containers.ContainerHealthStatusType },
			wantErr: `failed to get health status "app-0"`,
		},
		{
			name:    "write outcome",
			fault:   func(r *faultyRuntime) { r.failModify = true },
			wantErr: `failed to write health status "app-0"`,
		},
		{
			name:    "clean up outputs",
			fault:   func(r *faultyRuntime) { r.failCleanup = true },
			wantErr: "failed to clean up outputs",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			rt := newFaultyRuntime()
			seedRunningContainer(t, rt.state)
			test.fault(rt)

			ctrl := &HealthController{
				ProbeProvider: passingProbe,
				probes:        map[string]*probeRun{},
				notifyCh:      make(chan struct{}, 1),
			}

			t.Cleanup(func() {
				for _, run := range ctrl.probes {
					run.stop()
				}
			})

			err := ctrl.reconcile(t.Context(), rt, zaptest.NewLogger(t))
			assert.ErrorIs(t, err, errInjected)
			assert.ErrorContains(t, err, test.wantErr)
		})
	}
}

// TestHealthRunReturnsReconcileError covers Run giving up on a failed reconcile, with the default
// probe provider in place.
func TestHealthRunReturnsReconcileError(t *testing.T) {
	t.Parallel()

	rt := newFaultyRuntime()

	rt.failList = containers.ContainerInstanceStatusType
	rt.events <- controller.ReconcileEvent{}

	ctrl := &HealthController{}

	assert.ErrorIs(t, ctrl.Run(t.Context(), rt, zaptest.NewLogger(t)), errInjected)
	assert.NotNil(t, ctrl.ProbeProvider)
}

// TestHealthNotifyNeverBlocks covers an outcome changing while a reconcile is already due: the
// wake-up is dropped rather than blocking the probe, which goes on checking.
func TestHealthNotifyNeverBlocks(t *testing.T) {
	t.Parallel()

	var checks atomic.Int32

	ctrl := &HealthController{
		ProbeProvider: func(ProbeTarget) (Probe, io.Closer, error) {
			return func(context.Context) error {
				checks.Add(1)

				return nil
			}, nopCloser{}, nil
		},
		// Already full: a reconcile is due, and nothing drains it.
		notifyCh: make(chan struct{}, 1),
	}
	ctrl.notifyCh <- struct{}{}

	run := ctrl.start(zaptest.NewLogger(t), "app-0", desiredProbe{
		containerID: faultyTestContainer,
		target:      ProbeTarget{HealthCheck: faultyTestHealthCheck},
	}, optional.None[containers.ContainerHealthStatusSpec]())

	t.Cleanup(run.stop)

	// The first check turns the outcome healthy and wakes the controller; the ones after it only
	// happen if that did not block.
	assert.Eventually(t, func() bool { return checks.Load() >= 3 }, 5*time.Second, 10*time.Millisecond)

	result, decided := run.result()
	assert.True(t, decided)
	assert.True(t, result.Healthy)
	assert.Len(t, ctrl.notifyCh, 1)
}

// TestStatusHealthStatusReadError covers the health outcome of the latest instance failing to be
// read: the error is returned rather than the container reported without its health.
func TestStatusHealthStatusReadError(t *testing.T) {
	t.Parallel()

	rt := newFaultyRuntime()
	seedRunningContainer(t, rt.state)
	rt.failGet = containers.ContainerHealthStatusType

	spec, err := rt.state.Get(t.Context(), containers.NewContainerSpec(containers.NamespaceName, faultyTestContainer).Metadata())
	require.NoError(t, err)

	err = (&StatusController{}).reconcileContainerStatus(t.Context(), rt, zaptest.NewLogger(t), spec.(*containers.ContainerSpec)) //nolint:forcetypeassert,errcheck
	assert.ErrorIs(t, err, errInjected)
	assert.ErrorContains(t, err, `failed to get health status "app-0"`)
}
