// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	containersres "github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

func TestUnmountEphemeralPartitionWaitsForFinalizers(t *testing.T) {
	t.Setenv("PLATFORM", "container")

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	t.Cleanup(cancel)

	runtimeState, err := v1alpha1.NewState()
	require.NoError(t, err)

	resources := runtimeState.V1Alpha2().Resources()
	mountRequest := blockres.NewVolumeMountRequest(blockres.NamespaceName, constants.EphemeralPartitionLabel)

	require.NoError(t, resources.Create(ctx, mountRequest))
	require.NoError(t, resources.AddFinalizer(ctx, mountRequest.Metadata(), "mount-controller"))

	finalizerRemoved := make(chan error, 1)

	go func() {
		_, watchErr := resources.WatchFor(ctx, mountRequest.Metadata(), state.WithPhases(resource.PhaseTearingDown))
		if watchErr != nil {
			finalizerRemoved <- watchErr

			return
		}

		finalizerRemoved <- resources.RemoveFinalizer(ctx, mountRequest.Metadata(), "mount-controller")
	}()

	task, _ := v1alpha1.UnmountEphemeralPartition(runtime.SequenceReset, nil)
	err = task(ctx, log.New(io.Discard, "", 0), v1alpha1.NewRuntime(runtimeState, nil, nil))

	require.NoError(t, err)
	require.NoError(t, <-finalizerRemoved)

	_, err = resources.Get(ctx, mountRequest.Metadata())
	require.True(t, state.IsNotFoundError(err))
}

func TestTeardownContainerLifecycleRecordsDeadline(t *testing.T) {
	t.Setenv("PLATFORM", "container")

	for _, test := range []struct {
		name             string
		timeout          time.Duration
		existingDeadline time.Time
	}{
		{name: "first", timeout: 10 * time.Minute},
		{name: "shorter sequence timeout", timeout: time.Minute},
		{name: "taking over", timeout: 10 * time.Minute, existingDeadline: time.Now().Add(time.Minute).Truncate(time.Second)},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), test.timeout)
			t.Cleanup(cancel)

			runtimeState, err := v1alpha1.NewState()
			require.NoError(t, err)

			resources := runtimeState.V1Alpha2().Resources()
			lifecycle := containersres.NewContainerLifecycle(containersres.NamespaceName, containersres.ContainerLifecycleID)
			lifecycle.TypedSpec().ShutdownDeadline = test.existingDeadline

			require.NoError(t, resources.Create(ctx, lifecycle))
			require.NoError(t, resources.AddFinalizer(ctx, lifecycle.Metadata(), "runtime-controller"))

			if !test.existingDeadline.IsZero() {
				_, err = resources.Teardown(ctx, lifecycle.Metadata())
				require.NoError(t, err)
			}

			deadlinesC := watchShutdownDeadlines(ctx, t, resources, lifecycle.Metadata())

			start := time.Now()

			task, _ := v1alpha1.TeardownContainerLifecycle(runtime.SequenceReboot, nil)
			require.NoError(t, task(ctx, log.New(io.Discard, "", 0), v1alpha1.NewRuntime(runtimeState, nil, nil)))

			deadlines := <-deadlinesC
			require.NotEmpty(t, deadlines)

			if test.existingDeadline.IsZero() {
				timeout := min(test.timeout, constants.ContainerShutdownTimeout)

				assert.WithinRange(t, deadlines[len(deadlines)-1], start.Add(timeout-time.Second), time.Now().Add(timeout))
			} else {
				for _, deadline := range deadlines {
					assert.True(t, deadline.Equal(test.existingDeadline), "deadline %s, expected %s", deadline, test.existingDeadline)
				}
			}

			_, err = safe.StateGet[*containersres.ContainerLifecycle](ctx, resources, lifecycle.Metadata())
			require.True(t, state.IsNotFoundError(err))
		})
	}
}

// watchShutdownDeadlines returns every deadline the lifecycle resource has until it is destroyed, releasing
// it when it is torn down.
func watchShutdownDeadlines(ctx context.Context, t *testing.T, resources state.State, md *resource.Metadata) <-chan []time.Time {
	events := make(chan state.Event)
	require.NoError(t, resources.Watch(ctx, md, events))

	deadlinesC := make(chan []time.Time, 1)

	go func() {
		var deadlines []time.Time

		defer func() { deadlinesC <- deadlines }()

		for {
			var event state.Event

			select {
			case <-ctx.Done():
				return
			case event = <-events:
			}

			if event.Type == state.Destroyed {
				return
			}

			observed, ok := event.Resource.(*containersres.ContainerLifecycle)
			if !ok {
				continue
			}

			deadlines = append(deadlines, observed.TypedSpec().ShutdownDeadline)

			if observed.Metadata().Phase() == resource.PhaseTearingDown && observed.Metadata().Finalizers().Has("runtime-controller") {
				resources.RemoveFinalizer(ctx, md, "runtime-controller") //nolint:errcheck
			}
		}
	}()

	return deadlinesC
}
