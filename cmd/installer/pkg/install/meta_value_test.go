// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
)

func TestMetaValue(t *testing.T) {
	t.Parallel()

	var v install.MetaValue

	require.NoError(t, v.Parse("10=foo"))

	assert.Equal(t, uint8(10), v.Key)
	assert.Equal(t, "foo", v.Value)

	assert.Equal(t, "0xa=foo", v.String())

	var v2 install.MetaValue

	require.NoError(t, v2.Parse(v.String()))

	assert.Equal(t, v, v2)
}

func TestMetaValues(t *testing.T) {
	t.Parallel()

	var s install.MetaValues

	require.NoError(t, s.Set("10=foo"))
	require.NoError(t, s.Append("20=bar"))

	assert.Equal(t, "[0xa=foo,0x14=bar]", s.String())
}
