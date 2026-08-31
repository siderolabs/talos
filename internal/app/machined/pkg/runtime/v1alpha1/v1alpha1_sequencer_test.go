// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package v1alpha1

import (
	"log"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func TestNewSequencer(t *testing.T) {
	tests := []struct {
		name string
		want *Sequencer
	}{
		{
			name: "test",
			want: &Sequencer{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewSequencer(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewSequencer() = %v, want %v", got, tt.want)
			}
		})
	}
}

type resetOptions struct{}

func (resetOptions) GetGraceful() bool {
	return false
}

func (resetOptions) GetReboot() bool {
	return true
}

func (resetOptions) GetMode() machineapi.ResetRequest_WipeMode {
	return machineapi.ResetRequest_ALL
}

func (resetOptions) GetUserDisksToWipe() []string {
	return nil
}

func (resetOptions) GetSystemDiskTargets() []runtime.PartitionTarget {
	return nil
}

func (resetOptions) GetSystemDiskPaths() []string {
	return nil
}

func TestPreShutdownPhaseOrdering(t *testing.T) {
	t.Setenv("PLATFORM", "container")

	state, err := NewState()
	require.NoError(t, err)

	metalPlatform := &metal.Metal{}
	state.platform = metalPlatform
	state.machine.platform = metalPlatform

	rt := NewRuntime(
		state,
		NewEvents(1000, 10),
		logging.NewCircularBufferLoggingManager(log.New(t.Output(), "fallback logger: ", log.Flags())),
	)
	sequencer := NewSequencer()

	for _, tt := range []struct {
		name     string
		phases   []runtime.Phase
		expected bool
	}{
		{
			name:     "reboot",
			phases:   sequencer.Reboot(rt, &machineapi.RebootRequest{}),
			expected: true,
		},
		{
			name:     "reset",
			phases:   sequencer.Reset(rt, resetOptions{}),
			expected: true,
		},
		{
			name:     "shutdown",
			phases:   sequencer.Shutdown(rt, &machineapi.ShutdownRequest{}),
			expected: true,
		},
		{
			name:     "stage upgrade",
			phases:   sequencer.StageUpgrade(rt, &machineapi.UpgradeRequest{}),
			expected: true,
		},
		{
			name:     "upgrade",
			phases:   sequencer.Upgrade(rt, &machineapi.UpgradeRequest{}),
			expected: true,
		},
		{
			name: "forced reboot",
			phases: sequencer.Reboot(rt, &machineapi.RebootRequest{
				Mode: machineapi.RebootRequest_FORCE,
			}),
		},
		{
			name:   "maintenance upgrade",
			phases: sequencer.MaintenanceUpgrade(rt, &machineapi.UpgradeRequest{}),
		},
		{
			name:   "emergency cleanup",
			phases: sequencer.EmergencyVolumeCleanup(rt),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			names := make([]string, 0, len(tt.phases))
			for _, phase := range tt.phases {
				names = append(names, phase.Name)
			}

			preShutdown := slices.Index(names, "preShutdown")
			if !tt.expected {
				assert.Equal(t, -1, preShutdown)

				return
			}

			require.NotEqual(t, -1, preShutdown)
			require.Less(t, preShutdown+1, len(names))
			assert.Equal(t, "dbus", names[preShutdown+1])
		})
	}
}

func TestPhaseList_Append(t *testing.T) {
	t.Skip("temporarily disabling until reflect.DeepEqual responds as expected")

	type args struct {
		name  string
		tasks []runtime.TaskSetupFunc
	}

	tests := []struct {
		name string
		p    PhaseList
		args args
		want PhaseList
	}{
		{
			name: "test",
			p:    PhaseList{},
			args: args{
				name:  "mount",
				tasks: []runtime.TaskSetupFunc{KexecPrepare},
			},
			want: PhaseList{runtime.Phase{Name: "mount", Tasks: []runtime.TaskSetupFunc{KexecPrepare}}},
		},
	}

	cmp := func(a, b runtime.Phase) bool { return a.Name == b.Name }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.p = tt.p.Append(tt.args.name, tt.args.tasks...); !slices.EqualFunc(tt.p, tt.want, cmp) {
				t.Errorf("PhaseList.Append() = %v, want %v", tt.p, tt.want)
			}
		})
	}
}

func TestPhaseList_AppendWhen(t *testing.T) {
	t.Skip("temporarily disabling until reflect.DeepEqual responds as expected")

	type args struct {
		when  bool
		name  string
		tasks []runtime.TaskSetupFunc
	}

	tests := []struct {
		name string
		p    PhaseList
		args args
		want PhaseList
	}{
		{
			name: "true",
			p:    PhaseList{},
			args: args{
				when:  true,
				name:  "mount",
				tasks: []runtime.TaskSetupFunc{KexecPrepare},
			},
			want: PhaseList{runtime.Phase{Name: "mount", Tasks: []runtime.TaskSetupFunc{KexecPrepare}}},
		},
		{
			name: "false",
			p:    PhaseList{},
			args: args{
				when:  false,
				tasks: []runtime.TaskSetupFunc{KexecPrepare},
			},
			want: PhaseList{},
		},
	}

	cmp := func(a, b runtime.Phase) bool { return a.Name == b.Name }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.p = tt.p.AppendWhen(tt.args.when, tt.args.name, tt.args.tasks...); !slices.EqualFunc(tt.p, tt.want, cmp) {
				t.Errorf("PhaseList.AppendWhen() = %v, want %v", tt.p, tt.want)
			}
		})
	}
}
