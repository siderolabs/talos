// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package decoder_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader/internal/decoder"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

type Meta struct {
	MetaKind       string `yaml:"kind"`
	MetaAPIVersion string `yaml:"apiVersion,omitempty"`
}

func (m Meta) Kind() string {
	return m.MetaKind
}

func (m Meta) APIVersion() string {
	return m.MetaAPIVersion
}

type Mock struct {
	Meta
	Test bool `yaml:"test"`
}

func (m *Mock) Clone() config.Document {
	return m
}

type MockV2 struct {
	Meta
	Slice []Mock           `yaml:"slice"`
	Map   map[string]*Mock `yaml:"map"`
}

func (m *MockV2) Clone() config.Document {
	return m
}

type MockV3 struct {
	Meta
	Omit bool `yaml:"omit,omitempty"`
}

func (m *MockV3) Clone() config.Document {
	return m
}

type KubeletConfig struct {
	Meta
	v1alpha1.KubeletConfig `yaml:",inline"`
}

func (m *KubeletConfig) Clone() config.Document {
	return m
}

type MockUnstructured struct {
	Meta
	Pods []v1alpha1.Unstructured `yaml:"pods,omitempty"`
}

func (m *MockUnstructured) Clone() config.Document {
	return m
}

func init() {
	registry.Register("mock", func(version string) config.Document {
		switch version {
		case "v1alpha2":
			return &MockV2{}
		case "v1alpha3":
			return &MockV3{}
		}

		return &Mock{}
	})

	registry.Register("kubelet", func(string) config.Document {
		return &KubeletConfig{}
	})

	registry.Register("unstructured", func(string) config.Document {
		return &MockUnstructured{}
	})
}

func TestDecoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		source      []byte
		expected    []config.Document
		expectedErr string
	}{
		{
			name: "valid",
			source: []byte(`---
kind: mock
apiVersion: v1alpha1
test: true
`),
			expected: []config.Document{
				&Mock{
					Test: true,
				},
			},
			expectedErr: "",
		},
		{
			name: "missing kind",
			source: []byte(`---
apiVersion: v1alpha2
test: true
`),
			expected:    nil,
			expectedErr: "missing kind",
		},
		{
			name: "empty kind",
			source: []byte(`---
kind:
apiVersion: v1alpha2
test: true
`),
			expected:    nil,
			expectedErr: "missing kind",
		},
		{
			name: "tab instead of spaces",
			source: []byte(`---
kind: mock
apiVersion: v1alpha1
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
apiVersion: v1alpha1
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
apiVersion: v1alpha2
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
apiVersion: v1alpha2
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
apiVersion: v1alpha2
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
apiVersion: v1alpha2
slice:
  - test: true
map:
  first:
    test: true
  second:
    test: false
`),
			expected: []config.Document{
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
apiVersion: v1alpha1
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
apiVersion: v1alpha3
omit: false
`),
			expected:    nil,
			expectedErr: "",
		},
		{
			name:        "internal error",
			source:      []byte(":   \xea"),
			expected:    nil,
			expectedErr: "decode error: yaml: incomplete UTF-8 octet sequence",
		},
		{
			name: "unstructured config",
			source: []byte(`---
kind: unstructured
apiVersion: v1alpha1
pods:
 - destination: /var/local
   options:
     - rbind
     - rw
   source: /var/local
   something: 1.34
`),
			expected:    nil,
			expectedErr: "",
		},
		{
			name: "omit empty test",
			source: []byte(`---
kind: mock
apiVersion: v1alpha3
omit: false
`),
		},
		{
			name: "misspelled apiVersion",
			source: []byte(`---
apiversion: v1alpha1
kind: ExtensionServiceConfig
config:
    - name: nut-client
      configFiles:
          - content: MONITOR ${upsmonHost} 1 remote pass foo
            mountPath: /usr/local/etc/nut/upsmon.conf
`),
			expectedErr: "\"ExtensionServiceConfig\" \"\": not registered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := decoder.NewDecoder()
			actual, err := d.Decode(bytes.NewReader(tt.source))

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
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			contents, err := os.ReadFile(file)
			require.NoError(t, err)

			d := decoder.NewDecoder()
			_, err = d.Decode(bytes.NewReader(contents))

			assert.NoError(t, err)
		})
	}
}

func BenchmarkDecoderV1Alpha1Config(b *testing.B) {
	b.ReportAllocs()

	contents, err := os.ReadFile("testdata/controlplane.yaml")
	require.NoError(b, err)

	for range b.N {
		d := decoder.NewDecoder()
		_, err = d.Decode(bytes.NewReader(contents))

		assert.NoError(b, err)
	}
}
