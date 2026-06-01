// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package toml_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/toml"
)

//go:embed testdata/expected.toml
var expected []byte

func TestMerge(t *testing.T) {
	out, checksums, err := toml.Merge([]string{
		"testdata/1.toml",
		"testdata/2.toml",
		"testdata/3.toml",
	})
	require.NoError(t, err)

	assert.Equal(t, string(expected), string(out))
	assert.Contains(t, checksums, "testdata/1.toml")
	assert.Contains(t, checksums, "testdata/2.toml")
	assert.Contains(t, checksums, "testdata/3.toml")
}
