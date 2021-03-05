// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,scopelint,testpackage
package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
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
				tasks: []runtime.TaskSetupFunc{MountBootPartition},
			},
			want: PhaseList{runtime.Phase{Name: "mount", Tasks: []runtime.TaskSetupFunc{MountBootPartition}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.p = tt.p.Append(tt.args.name, tt.args.tasks...); !reflect.DeepEqual(tt.p, tt.want) {
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
				tasks: []runtime.TaskSetupFunc{MountBootPartition},
			},
			want: PhaseList{runtime.Phase{Name: "mount", Tasks: []runtime.TaskSetupFunc{MountBootPartition}}},
		},
		{
			name: "false",
			p:    PhaseList{},
			args: args{
				when:  false,
				tasks: []runtime.TaskSetupFunc{MountBootPartition},
			},
			want: PhaseList{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.p = tt.p.AppendWhen(tt.args.when, tt.args.name, tt.args.tasks...); !reflect.DeepEqual(tt.p, tt.want) {
				t.Errorf("PhaseList.AppendWhen() = %v, want %v", tt.p, tt.want)
			}
		})
	}
}

func TestSequencer_Initialize(t *testing.T) {
	type args struct {
		r runtime.Runtime
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Initialize(tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Initialize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSequencer_Install(t *testing.T) {
	type args struct {
		r runtime.Runtime
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Install(tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Install() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSequencer_Boot(t *testing.T) {
	type args struct {
		r runtime.Runtime
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Boot(tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Boot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSequencer_Reboot(t *testing.T) {
	type args struct {
		r runtime.Runtime
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Reboot(tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Reboot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSequencer_Reset(t *testing.T) {
	type args struct {
		r  runtime.Runtime
		in runtime.ResetOptions
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Reset(tt.args.r, tt.args.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Reset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSequencer_Shutdown(t *testing.T) {
	type args struct {
		r runtime.Runtime
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Shutdown(tt.args.r); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Shutdown() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSequencer_Upgrade(t *testing.T) {
	type args struct {
		r  runtime.Runtime
		in *machine.UpgradeRequest
	}

	tests := []struct {
		name string
		s    *Sequencer
		args args
		want []runtime.Phase
	}{
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Sequencer{}
			if got := s.Upgrade(tt.args.r, tt.args.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sequencer.Upgrade() = %v, want %v", got, tt.want)
			}
		})
	}
}
