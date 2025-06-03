// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package yamlutils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/yamlutils"
)

func TestStringBytes(t *testing.T) {
	t.Parallel()

	type sbStruct struct {
		Field yamlutils.StringBytes `yaml:"field"`
	}

	for _, test := range []struct {
		name string

		in       any
		expected string

		empty func() any

		// extraMarshaled is a list of strings that should be unmarshaled from YAML into the same `in`
		extraMarshaled []string
	}{
		{
			name: "simple",
			in:   &sbStruct{yamlutils.StringBytes([]byte("abcde"))},

			expected: "field: abcde\n",
			empty: func() any {
				return &sbStruct{}
			},
			extraMarshaled: []string{
				"field:\n - 0x61\n - 0x62\n - 0x63\n - 0x64\n - 0x65\n",
				"field:\n    - 97\n    - 98\n    - 99\n    - 100\n    - 101\n",
			},
		},
		{
			name: "empty",
			in:   &sbStruct{yamlutils.StringBytes([]byte{})},

			expected: "field: \"\"\n",
			empty: func() any {
				return &sbStruct{}
			},
		},
		{
			name: "invalid utf8",
			in:   &sbStruct{yamlutils.StringBytes([]byte{0xff})},

			expected: "field:\n    - 255\n",
			empty: func() any {
				return &sbStruct{}
			},

			extraMarshaled: []string{
				"field:\n - 0xff\n",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			out, err := yaml.Marshal(test.in)
			require.NoError(t, err)

			assert.Equal(t, test.expected, string(out))

			back := test.empty()

			err = yaml.Unmarshal(out, back)
			require.NoError(t, err)

			assert.Equal(t, test.in, back)

			for _, extra := range test.extraMarshaled {
				back := test.empty()

				err = yaml.Unmarshal([]byte(extra), back)
				require.NoError(t, err)

				assert.Equal(t, test.in, back)
			}
		})
	}
}
