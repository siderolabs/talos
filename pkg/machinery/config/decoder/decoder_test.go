// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package decoder_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/decoder"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type Mock struct {
	Test bool `yaml:"test"`
}

type MockV2 struct {
	Slice []Mock           `yaml:"slice"`
	Map   map[string]*Mock `yaml:"map"`
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

func TestDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      []byte
		expected    []interface{}
		expectedErr string
	}{
		{
			name: "valid",
			source: []byte(`---
kind: mock
version: v1alpha1
spec:
  test: true
`),
			expected: []interface{}{
				&Mock{
					Test: true,
				},
			},
			expectedErr: "",
		},
		{
			name: "missing kind",
			source: []byte(`---
version: v1alpha1
spec:
  test: true
`),
			expected:    nil,
			expectedErr: "missing kind",
		},
		{
			name: "empty kind",
			source: []byte(`---
kind:
version: v1alpha1
spec:
  test: true
`),
			expected:    nil,
			expectedErr: "missing kind",
		},
		{
			name: "missing version",
			source: []byte(`---
kind: mock
spec:
  test: true
`),
			expected:    nil,
			expectedErr: "missing version",
		},
		{
			name: "empty version",
			source: []byte(`---
kind: mock
version:
spec:
  test: true
`),
			expected:    nil,
			expectedErr: "missing version",
		},
		{
			name: "missing spec",
			source: []byte(`---
kind: mock
version: v1alpha1
`),
			expected:    nil,
			expectedErr: "missing spec",
		},
		{
			name: "empty spec",
			source: []byte(`---
kind: mock
version: v1alpha1
spec:
`),
			expected:    nil,
			expectedErr: "missing spec content",
		},
		{
			name: "tab instead of spaces",
			source: []byte(`---
kind: mock
version: v1alpha1
spec:
	test: true
`),
			expected:    nil,
			expectedErr: "decode error: yaml: line 5: found character that cannot start any token",
		},
		{
			name: "extra field",
			source: []byte(`---
kind: mock
version: v1alpha1
spec:
  test: true
  extra: fail
`),
			expected:    nil,
			expectedErr: "unknown keys found during decoding:\nextra: fail\n",
		},
		{
			name: "extra fields in map",
			source: []byte(`---
kind: mock
version: v1alpha2
spec:
  map:
    first:
      test: true
      extra: me
`),
			expected:    nil,
			expectedErr: "unknown keys found during decoding:\nmap:\n    first:\n        extra: me\n",
		},
		{
			name: "extra fields in slice",
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
			expected:    nil,
			expectedErr: "unknown keys found during decoding:\nslice:\n    - fields: here\n      more: extra\n      not: working\n",
		},
		{
			name: "extra zero fields in map",
			source: []byte(`---
kind: mock
version: v1alpha2
spec:
  map:
    second:
      a:
        b: {}
`),
			expected:    nil,
			expectedErr: "unknown keys found during decoding:\nmap:\n    second:\n        a:\n            b: {}\n",
		},
		{
			name: "valid nested",
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
			expected: []interface{}{
				&MockV2{
					Map: map[string]*Mock{
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
			expectedErr: "",
		},
		{
			name: "kubelet config",
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
			expected:    nil,
			expectedErr: "",
		},
		{
			name: "omit empty test",
			source: []byte(`---
kind: mock
version: v1alpha3
spec:
  omit: false
`),
			expected:    nil,
			expectedErr: "",
		},
		{
			name:        "internal error",
			source:      []byte(":   \xea"),
			expected:    nil,
			expectedErr: "recovered: internal error: attempted to parse unknown event (please report): none",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := decoder.NewDecoder(tt.source)
			actual, err := d.Decode()
			if tt.expected != nil {
				assert.Equal(t, tt.expected, actual)
			}
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}

func TestDecoderV1Alpha1Config(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join("testdata", "*.yaml"))
	require.NoError(t, err)

	for _, file := range files {
		file := file

		t.Run(file, func(t *testing.T) {
			t.Parallel()

			contents, err := ioutil.ReadFile(file)
			require.NoError(t, err)

			d := decoder.NewDecoder(contents)
			_, err = d.Decode()

			assert.NoError(t, err)
		})
	}
}

func BenchmarkDecoderV1Alpha1Config(b *testing.B) {
	b.ReportAllocs()

	contents, err := ioutil.ReadFile("testdata/controlplane.yaml")
	require.NoError(b, err)

	for i := 0; i < b.N; i++ {
		d := decoder.NewDecoder(contents)
		_, err = d.Decode()

		assert.NoError(b, err)
	}
}
