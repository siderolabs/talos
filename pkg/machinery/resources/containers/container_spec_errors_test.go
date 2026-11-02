// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

var errInjected = errors.New("injected")

// faultyReader reads from a state, except for the resource types it fails reading.
type faultyReader struct {
	state.State

	failGet  resource.Type
	failList resource.Type
}

func (r faultyReader) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	if ptr.Type() == r.failGet {
		return nil, errInjected
	}

	return r.State.Get(ctx, ptr, opts...)
}

func (r faultyReader) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	if kind.Type() == r.failList {
		return resource.List{}, errInjected
	}

	return r.State.List(ctx, kind, opts...)
}

// TestContainersReadyReadErrors covers a dependency whose resources cannot be read: unlike a missing
// resource, which only means waiting, the error is returned rather than read as not ready.
func TestContainersReadyReadErrors(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		failGet  resource.Type
		failList resource.Type
		wantErr  string
	}{
		{
			name:    "container status",
			failGet: containers.ContainerStatusType,
			wantErr: `failed to get container status "other"`,
		},
		{
			name:     "instance status",
			failList: containers.ContainerInstanceStatusType,
			wantErr:  `failed to get instance status for container "other"`,
		},
		{
			name:    "container spec",
			failGet: containers.ContainerSpecType,
			wantErr: `failed to get container spec "other"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(inmem.NewState())

			// A running, healthy dependency, so that every read is reached.
			status := containers.NewContainerStatus(containers.NamespaceName, "other")
			status.TypedSpec().Health = containers.ContainerHealthHealthy
			status.TypedSpec().State = containers.ContainerStateRunning
			require.NoError(t, st.Create(ctx, status))

			instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID("other", 0))
			instanceStatus.TypedSpec().ContainerID = "other"
			instanceStatus.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
			instanceStatus.TypedSpec().StartedAt = time.Now().Add(-time.Hour)
			instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, "other")
			require.NoError(t, st.Create(ctx, instanceStatus))

			_, _, err := containers.ContainerDependsOnSpec{
				Containers: []containers.ContainerDependencySpec{{Name: "other"}},
			}.ContainersReady(ctx, faultyReader{State: st, failGet: test.failGet, failList: test.failList})

			assert.ErrorIs(t, err, errInjected)
			assert.ErrorContains(t, err, test.wantErr)
		})
	}
}
