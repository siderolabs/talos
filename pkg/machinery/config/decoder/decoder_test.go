// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint
package decoder_test

import (
	"reflect"
	"testing"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/decoder"
)

type Mock struct {
	Test bool `yaml:"test"`
}

func init() {
	config.Register("mock", func(string) interface{} {
		return &Mock{}
	})
}

func TestDecoder_Decode(t *testing.T) {
	type fields struct {
		source []byte
	}

	tests := []struct {
		name    string
		fields  fields
		want    []interface{}
		wantErr bool
	}{
		{
			name: "valid",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha1
spec:
  test: true
`),
			},
			want: []interface{}{
				&Mock{
					Test: true,
				},
			},
			wantErr: false,
		},
		{
			name: "missing kind",
			fields: fields{
				source: []byte(`---
version: v1alpha1
spec:
  test: true
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty kind",
			fields: fields{
				source: []byte(`---
kind:
version: v1alpha1
spec:
  test: true
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing version",
			fields: fields{
				source: []byte(`---
kind: mock
spec:
  test: true
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty version",
			fields: fields{
				source: []byte(`---
kind: mock
version:
spec:
  test: true
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing spec",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha1
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty spec",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha1
spec:
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "tab instead of spaces",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha1
spec:
	test: true
`),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := decoder.NewDecoder(tt.fields.source)
			got, err := d.Decode()
			if (err != nil) != tt.wantErr {
				t.Errorf("Decoder.Decode() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decoder.Decode() = %v, want %v", got, tt.want)
			}
		})
	}
}
