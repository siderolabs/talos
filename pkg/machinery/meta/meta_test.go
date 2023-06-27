// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package meta_test

import (
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

	values := make(meta.Values, 2)

	require.NoError(t, values[0].Parse("10=foo"))
	require.NoError(t, values[1].Parse("0xb=bar"))

	encoded := values.Encode()

	decoded, err := meta.DecodeValues(encoded)
	require.NoError(t, err)

	assert.Equal(t, values, decoded)
}
