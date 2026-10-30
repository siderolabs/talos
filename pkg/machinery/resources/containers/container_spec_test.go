// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"cmp"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
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
		name            string
		containers      []string
		setup           func(*containers.ContainerStatus)
		setupInstance   func(*containers.ContainerInstanceStatus)
		want            []string
		wantWakeUpAfter bool
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
			name:       "healthy but just started",
			containers: []string{"other"},
			setup: func(status *containers.ContainerStatus) {
				status.TypedSpec().Health = containers.ContainerHealthHealthy
				status.TypedSpec().State = containers.ContainerStateRunning
			},
			setupInstance: func(status *containers.ContainerInstanceStatus) {
				status.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
				status.TypedSpec().StartedAt = time.Now()
			},
			want:            []string{"container: other"},
			wantWakeUpAfter: true,
		},
		{
			name:       "healthy and stable",
			containers: []string{"other"},
			setup: func(status *containers.ContainerStatus) {
				status.TypedSpec().Health = containers.ContainerHealthHealthy
				status.TypedSpec().State = containers.ContainerStateRunning
			},
			setupInstance: func(status *containers.ContainerInstanceStatus) {
				status.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
				status.TypedSpec().StartedAt = time.Now().Add(-time.Hour)
			},
			want: nil,
		},
		{
			name:       "stopping keeps its health",
			containers: []string{"other"},
			setup: func(status *containers.ContainerStatus) {
				// What StatusController writes for an instance that is on its way out: the health it had
				// last, carried over, with the state saying what is actually happening to it.
				status.TypedSpec().Health = containers.ContainerHealthHealthy
				status.TypedSpec().State = containers.ContainerStateStopping
			},
			setupInstance: func(status *containers.ContainerInstanceStatus) {
				status.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
				status.TypedSpec().StartedAt = time.Now().Add(-time.Hour)
			},
			want: []string{"container: other"},
		},
		{
			name:       "healthy but no instance status",
			containers: []string{"other"},
			setup: func(status *containers.ContainerStatus) {
				status.TypedSpec().Health = containers.ContainerHealthHealthy
				status.TypedSpec().State = containers.ContainerStateRunning
			},
			want: []string{"container: other"},
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

			if tt.setupInstance != nil {
				instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID("other", 0))
				instanceStatus.TypedSpec().ContainerID = "other"
				instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, "other")
				tt.setupInstance(instanceStatus)
				require.NoError(t, st.Create(ctx, instanceStatus))
			}

			got, wakeUpAfter, err := containers.ContainerDependsOnSpec{Containers: xslices.Map(tt.containers, func(name string) containers.ContainerDependencySpec {
				return containers.ContainerDependencySpec{Name: name}
			})}.ContainersReady(ctx, st)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)

			_, wakeUpAfterSet := wakeUpAfter.Get()
			assert.Equal(t, tt.wantWakeUpAfter, wakeUpAfterSet)
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
				Containers: []containers.ContainerDependencySpec{{Name: "other"}},
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
		Containers: []containers.ContainerDependencySpec{{Name: "other"}},
	}

	waitingFor, wakeUpAfter, err := dependsOn.Ready(ctx, st)
	require.NoError(t, err)
	assert.Equal(t, []string{"network: addresses", "time", "container: other"}, waitingFor)

	_, wakeUpAfterSet := wakeUpAfter.Get()
	assert.False(t, wakeUpAfterSet)
}

func TestExecutionHash(t *testing.T) {
	t.Parallel()

	base := containers.ContainerSpecSpec{
		Image:       containers.ContainerImageSpec{Ref: "docker.io/library/nginx:latest"},
		Environment: []string{"A=1"},
	}

	hash := base.ExecutionHash()

	same := base
	same.Restart = containers.ContainerRestartSpec{Policy: containers.ContainerRestartPolicyNever, Delay: time.Minute}
	same.DependsOn = containers.ContainerDependsOnSpec{Containers: []containers.ContainerDependencySpec{{Name: "db"}}}

	assert.Equal(t, hash, same.ExecutionHash())

	for _, mutate := range []func(*containers.ContainerSpecSpec){
		func(spec *containers.ContainerSpecSpec) { spec.Image.Ref = "docker.io/library/nginx:1.27" },
		func(spec *containers.ContainerSpecSpec) { spec.Environment = []string{"A=2"} },
		func(spec *containers.ContainerSpecSpec) { spec.Args = []string{"-v"} },
		func(spec *containers.ContainerSpecSpec) { spec.Network.HostNetwork = true },
		func(spec *containers.ContainerSpecSpec) {
			spec.Mounts = []containers.ContainerMountSpec{{Kind: containers.MountKindTmpfs, Destination: "/tmp"}}
		},
	} {
		changed := base
		mutate(&changed)

		assert.NotEqual(t, hash, changed.ExecutionHash())
	}
}

func TestContainersReadySucceeded(t *testing.T) {
	t.Parallel()

	initSpec := containers.ContainerSpecSpec{
		Image:   containers.ContainerImageSpec{Ref: "ghcr.io/example/init:v1.0.0"},
		Restart: containers.ContainerRestartSpec{Policy: containers.ContainerRestartPolicyNever},
	}

	for _, test := range []struct {
		name       string
		restart    containers.ContainerRestartSpec
		phase      containers.ContainerInstancePhase
		exitCode   int32
		hash       string
		charged    bool
		noInstance bool
		succeeded  bool
	}{
		{name: "completed", restart: initSpec.Restart, phase: containers.ContainerInstancePhaseTerminated, succeeded: true},
		{name: "on-failure completed", restart: containers.ContainerRestartSpec{Policy: containers.ContainerRestartPolicyOnFailure}, phase: containers.ContainerInstancePhaseTerminated, succeeded: true},
		{name: "not run yet", noInstance: true},
		{name: "running", restart: initSpec.Restart, phase: containers.ContainerInstancePhaseRunning},
		{name: "failed", restart: initSpec.Restart, phase: containers.ContainerInstancePhaseTerminated, exitCode: 3},
		{name: "failed start", restart: initSpec.Restart, phase: containers.ContainerInstancePhaseFailed},
		{name: "run of a changed configuration", restart: initSpec.Restart, phase: containers.ContainerInstancePhaseTerminated, hash: "previous"},
		{name: "restart pending", restart: initSpec.Restart, phase: containers.ContainerInstancePhaseTerminated, charged: true},
		{name: "restarted anyway", restart: containers.ContainerRestartSpec{}, phase: containers.ContainerInstancePhaseTerminated},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			spec := initSpec
			spec.Restart = test.restart

			containerSpec := containers.NewContainerSpec(containers.NamespaceName, "init")
			*containerSpec.TypedSpec() = spec
			require.NoError(t, st.Create(ctx, containerSpec))

			if !test.noInstance {
				instanceSpec := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID("init", 0))
				instanceSpec.TypedSpec().ContainerID = "init"
				instanceSpec.TypedSpec().ExecutionHash = cmp.Or(test.hash, spec.ExecutionHash())
				instanceSpec.Metadata().Labels().Set(containers.ContainerSpecIdLabel, "init")
				require.NoError(t, st.Create(ctx, instanceSpec))

				instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID("init", 0))
				instanceStatus.TypedSpec().ContainerID = "init"
				instanceStatus.TypedSpec().Phase = test.phase
				instanceStatus.TypedSpec().ExitCode = test.exitCode
				instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, "init")
				require.NoError(t, st.Create(ctx, instanceStatus))
			}

			runStatus := containers.NewContainerRunStatus(containers.NamespaceName, "init")
			runStatus.TypedSpec().Restarted = test.charged
			require.NoError(t, st.Create(ctx, runStatus))

			waitingFor, _, err := containers.ContainerDependsOnSpec{
				Containers: []containers.ContainerDependencySpec{{Name: "init", Condition: containers.ContainerDependencyConditionSucceeded}},
			}.ContainersReady(ctx, st)
			require.NoError(t, err)

			if test.succeeded {
				assert.Empty(t, waitingFor)
			} else {
				assert.Equal(t, []string{"container: init to succeed"}, waitingFor)
			}
		})
	}
}
