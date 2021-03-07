// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint
package machine_test

import (
	"reflect"
	"testing"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

func TestMachineType_String(t *testing.T) {
	tests := []struct {
		name string
		t    machine.Type
		want string
	}{
		{
			name: "init",
			t:    machine.TypeInit,
			want: "init",
		},
		{
			name: "controlplane",
			t:    machine.TypeControlPlane,
			want: "controlplane",
		},
		{
			name: "join",
			t:    machine.TypeJoin,
			want: "join",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.String(); got != tt.want {
				t.Errorf("MachineType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMachineType(t *testing.T) {
	type args struct {
		t string
	}

	tests := []struct {
		name    string
		args    args
		want    machine.Type
		wantErr bool
	}{
		{
			name:    "init",
			args:    args{"init"},
			want:    machine.TypeInit,
			wantErr: false,
		},
		{
			name:    "controlplane",
			args:    args{"controlplane"},
			want:    machine.TypeControlPlane,
			wantErr: false,
		},
		{
			name:    "join",
			args:    args{"join"},
			want:    machine.TypeJoin,
			wantErr: false,
		},
		{
			name:    "invalid",
			args:    args{"invalid"},
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := machine.ParseType(tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMachineType() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMachineType() = %v, want %v", got, tt.want)
			}
		})
	}
}
