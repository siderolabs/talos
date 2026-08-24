// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

func TestTimeReady(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name  string
		time  bool
		setup func(*timeres.Status)
		want  bool
	}{
		{
			name: "time not declared",
			time: false,
			want: true,
		},
		{
			name: "status missing",
			time: true,
			want: false,
		},
		{
			name: "synced true",
			time: true,
			setup: func(status *timeres.Status) {
				status.TypedSpec().Synced = true
			},
			want: true,
		},
		{
			name: "sync disabled blocks forever",
			time: true,
			setup: func(status *timeres.Status) {
				status.TypedSpec().SyncDisabled = true
			},
			want: false,
		},
		{
			name: "synced false sync disabled false",
			time: true,
			setup: func(status *timeres.Status) {
				// Both false by default
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			if tt.setup != nil {
				status := timeres.NewStatus()
				tt.setup(status)
				require.NoError(t, st.Create(ctx, status))
			}

			got, err := containers.ContainerDependsOnSpec{Time: tt.time}.TimeReady(ctx, st)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNetworkConditionMet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(*network.Status)
		condition string
		want      bool
	}{
		{
			name:      "nil status",
			setup:     nil,
			condition: "addresses",
			want:      false,
		},
		{
			name: "addresses ready",
			setup: func(status *network.Status) {
				status.TypedSpec().AddressReady = true
			},
			condition: "addresses",
			want:      true,
		},
		{
			name: "addresses not ready",
			setup: func(status *network.Status) {
				status.TypedSpec().AddressReady = false
			},
			condition: "addresses",
			want:      false,
		},
		{
			name: "connectivity ready",
			setup: func(status *network.Status) {
				status.TypedSpec().ConnectivityReady = true
			},
			condition: "connectivity",
			want:      true,
		},
		{
			name: "connectivity not ready",
			setup: func(status *network.Status) {
				status.TypedSpec().ConnectivityReady = false
			},
			condition: "connectivity",
			want:      false,
		},
		{
			name: "hostname ready",
			setup: func(status *network.Status) {
				status.TypedSpec().HostnameReady = true
			},
			condition: "hostname",
			want:      true,
		},
		{
			name: "hostname not ready",
			setup: func(status *network.Status) {
				status.TypedSpec().HostnameReady = false
			},
			condition: "hostname",
			want:      false,
		},
		{
			name: "etcfiles ready",
			setup: func(status *network.Status) {
				status.TypedSpec().EtcFilesReady = true
			},
			condition: "etcfiles",
			want:      true,
		},
		{
			name: "etcfiles not ready",
			setup: func(status *network.Status) {
				status.TypedSpec().EtcFilesReady = false
			},
			condition: "etcfiles",
			want:      false,
		},
		{
			name: "unknown condition",
			setup: func(status *network.Status) {
				status.TypedSpec().AddressReady = true
			},
			condition: "unknown",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var status *network.Status
			if tt.setup != nil {
				status = network.NewStatus(network.NamespaceName, network.StatusID)
				tt.setup(status)
			}

			got := containers.ContainerDependsOnSpec{}.NetworkConditionMet(status, tt.condition)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContainersReady(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name       string
		containers []string
		setup      func(*containers.ContainerStatus)
		want       []string
	}{
		{
			name:       "no containers declared",
			containers: nil,
			want:       nil,
		},
		{
			name:       "status missing",
			containers: []string{"other"},
			want:       []string{"container: other"},
		},
		{
			name:       "healthy",
			containers: []string{"other"},
			setup: func(status *containers.ContainerStatus) {
				status.TypedSpec().Health = containers.ContainerHealthHealthy
			},
			want: nil,
		},
		{
			name:       "degraded",
			containers: []string{"other"},
			setup: func(status *containers.ContainerStatus) {
				status.TypedSpec().Health = containers.ContainerHealthDegraded
			},
			want: []string{"container: other"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			if tt.setup != nil {
				status := containers.NewContainerStatus(containers.NamespaceName, "other")
				tt.setup(status)
				require.NoError(t, st.Create(ctx, status))
			}

			got, err := containers.ContainerDependsOnSpec{Containers: tt.containers}.ContainersReady(ctx, st)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReady(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	require.NoError(t, st.Create(ctx, networkStatus))

	timeStatus := timeres.NewStatus()
	timeStatus.TypedSpec().Synced = true
	require.NoError(t, st.Create(ctx, timeStatus))

	existingPath := filepath.Join(t.TempDir(), "exists")
	require.NoError(t, os.WriteFile(existingPath, nil, 0o644))

	missingPath := filepath.Join(t.TempDir(), "missing")

	for _, tt := range []struct {
		name            string
		dependsOn       containers.ContainerDependsOnSpec
		wantWaitingFor  []string
		wantWakeUpAfter bool
	}{
		{
			name:      "no gates declared",
			dependsOn: containers.ContainerDependsOnSpec{},
		},
		{
			name: "network condition met",
			dependsOn: containers.ContainerDependsOnSpec{
				Networks: []string{"addresses"},
			},
		},
		{
			name: "network condition unmet",
			dependsOn: containers.ContainerDependsOnSpec{
				Networks: []string{"connectivity"},
			},
			wantWaitingFor: []string{"network: connectivity"},
		},
		{
			name: "time condition met",
			dependsOn: containers.ContainerDependsOnSpec{
				Time: true,
			},
		},
		{
			name: "path exists",
			dependsOn: containers.ContainerDependsOnSpec{
				Paths: []string{existingPath},
			},
			wantWakeUpAfter: true,
		},
		{
			name: "path missing",
			dependsOn: containers.ContainerDependsOnSpec{
				Paths: []string{missingPath},
			},
			wantWaitingFor:  []string{"path: " + missingPath},
			wantWakeUpAfter: true,
		},
		{
			name: "container dependency unmet",
			dependsOn: containers.ContainerDependsOnSpec{
				Containers: []string{"other"},
			},
			wantWaitingFor: []string{"container: other"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			waitingFor, wakeUpAfter, err := tt.dependsOn.Ready(ctx, st)
			require.NoError(t, err)
			assert.Equal(t, tt.wantWaitingFor, waitingFor)

			_, wakeUpAfterSet := wakeUpAfter.Get()
			assert.Equal(t, tt.wantWakeUpAfter, wakeUpAfterSet)
		})
	}
}

func TestReadyMissingStatuses(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// No network or time Status resources exist in this state, exercising the not-found path.
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	dependsOn := containers.ContainerDependsOnSpec{
		Networks:   []string{"addresses"},
		Time:       true,
		Containers: []string{"other"},
	}

	waitingFor, wakeUpAfter, err := dependsOn.Ready(ctx, st)
	require.NoError(t, err)
	assert.Equal(t, []string{"network: addresses", "time", "container: other"}, waitingFor)

	_, wakeUpAfterSet := wakeUpAfter.Get()
	assert.False(t, wakeUpAfterSet)
}
