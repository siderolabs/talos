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
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type Mock struct {
	Test bool `yaml:"test"`
}

type MockV2 struct {
	Slice []Mock          `yaml:"slice"`
	Map   map[string]Mock `yaml:"map"`
}

type MockV3 struct {
	Omit bool `yaml:"omit,omitempty"`
}

func init() {
	config.Register("mock", func(version string) interface{} {
		switch version {
		case "v1alpha2":
			return &MockV2{}
		case "v1alpha3":
			return &MockV3{}
		}

		return &Mock{}
	})

	config.Register("kubelet", func(string) interface{} {
		return &v1alpha1.KubeletConfig{}
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
		{
			name: "extra field",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha1
spec:
  test: true
  extra: fail
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "extra fields in map",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha2
spec:
  map:
    first:
      test: true
      extra: me
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "extra fields in slice",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha2
spec:
  slice:
    - test: true
      not: working
      more: extra
      fields: here
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "extra zero fields in map",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha2
spec:
  map:
    second:
      a:
        b: {}
`),
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "valid nested",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha2
spec:
  slice:
    - test: true
  map:
    first:
      test: true
    second:
      test: false
`),
			},
			want: []interface{}{
				&MockV2{
					Map: map[string]Mock{
						"first": {
							Test: true,
						},
						"second": {
							Test: false,
						},
					},
					Slice: []Mock{
						{Test: true},
					},
				},
			},
		},
		{
			name: "kubelet config",
			fields: fields{
				source: []byte(`---
kind: kubelet
version: v1alpha1
spec:
  extraMounts:
   - destination: /var/local
     options:
       - rbind
       - rshared
       - rw
     source: /var/local
`),
			},
		},
		{
			name: "omit empty test",
			fields: fields{
				source: []byte(`---
kind: mock
version: v1alpha3
spec:
  omit: false
`),
			},
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
			if tt.want != nil {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("Decoder.Decode() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
