// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/meta"
)

func TestValue(t *testing.T) {
	t.Parallel()

	var v meta.Value

	require.NoError(t, v.Parse("10=foo"))

	assert.Equal(t, uint8(10), v.Key)
	assert.Equal(t, "foo", v.Value)

	assert.Equal(t, "0xa=foo", v.String())

	var v2 meta.Value

	require.NoError(t, v2.Parse(v.String()))

	assert.Equal(t, v, v2)
}

func TestEncodeDecodeValues(t *testing.T) {
	t.Parallel()

	for _, allowGzip := range []bool{false, true} {
		t.Run(fmt.Sprintf("allowGzip=%v", allowGzip), func(t *testing.T) {
			t.Parallel()

			for _, test := range []struct {
				name string

				values []string

				expectedEncodedSize int
				expectedGzippedSize int
			}{
				{
					name: "empty",
				},
				{
					name: "simple",
					values: []string{
						"10=foo",
						"0xb=bar",
					},

					expectedEncodedSize: 20,
					expectedGzippedSize: 20,
				},
				{
					name: "huge",
					values: []string{
						"10=" + strings.Repeat("foobar", 256),
						"0xb=" + strings.Repeat("baz", 256),
					},

					expectedEncodedSize: 3084,
					expectedGzippedSize: 80,
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()

					values := make(meta.Values, len(test.values))

					for i, v := range test.values {
						require.NoError(t, values[i].Parse(v))
					}

					if len(values) == 0 {
						values = nil
					}

					encoded := values.Encode(allowGzip)

					switch {
					case test.expectedEncodedSize > 0 && !allowGzip:
						assert.Equal(t, test.expectedEncodedSize, len(encoded))
					case test.expectedGzippedSize > 0 && allowGzip:
						assert.Equal(t, test.expectedGzippedSize, len(encoded))
					}

					decoded, err := meta.DecodeValues(encoded)
					require.NoError(t, err)

					assert.Equal(t, values, decoded)
				})
			}
		})
	}
}
