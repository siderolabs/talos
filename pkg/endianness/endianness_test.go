// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint: dupl,scopelint
package endianness_test

import (
	"reflect"
	"testing"

	"github.com/talos-systems/talos/pkg/endianness"
)

var (
	uuid   = []byte{15, 198, 61, 175, 132, 131, 71, 114, 142, 121, 61, 105, 216, 71, 125, 228}
	middle = []byte{175, 61, 198, 15, 131, 132, 114, 71, 142, 121, 61, 105, 216, 71, 125, 228}
)

func TestToMiddleEndian(t *testing.T) {
	type args struct {
		data []byte
	}

	tests := []struct {
		name    string
		args    args
		wantB   []byte
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				data: uuid,
			},
			wantB:   middle,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotB, err := endianness.ToMiddleEndian(tt.args.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("ToMiddleEndian() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotB, tt.wantB) {
				t.Errorf("ToMiddleEndian() = %v, want %v", gotB, tt.wantB)
			}
		})
	}
}

func TestFromMiddleEndian(t *testing.T) {
	type args struct {
		data []byte
	}

	tests := []struct {
		name    string
		args    args
		wantB   []byte
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				data: middle,
			},
			wantB:   uuid,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotB, err := endianness.FromMiddleEndian(tt.args.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("FromMiddleEndian() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(gotB, tt.wantB) {
				t.Errorf("FromMiddleEndian() = %v, want %v", gotB, tt.wantB)
			}
		})
	}
}
