// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint,testpackage
package v1alpha1

import (
	"reflect"
	"slices"
	"testing"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
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
