// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: scopelint,dupl
package runtime_test

import (
	"reflect"
	"testing"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

func TestMachineType_String(t *testing.T) {
	tests := []struct {
		name string
		t    runtime.MachineType
		want string
	}{
		{
			name: "init",
			t:    runtime.MachineTypeInit,
			want: "init",
		},
		{
			name: "controlplane",
			t:    runtime.MachineTypeControlPlane,
			want: "controlplane",
		},
		{
			name: "join",
			t:    runtime.MachineTypeJoin,
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
		want    runtime.MachineType
		wantErr bool
	}{
		{
			name:    "init",
			args:    args{"init"},
			want:    runtime.MachineTypeInit,
			wantErr: false,
		},
		{
			name:    "controlplane",
			args:    args{"controlplane"},
			want:    runtime.MachineTypeControlPlane,
			wantErr: false,
		},
		{
			name:    "join",
			args:    args{"join"},
			want:    runtime.MachineTypeJoin,
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
			got, err := runtime.ParseMachineType(tt.args.t)
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
