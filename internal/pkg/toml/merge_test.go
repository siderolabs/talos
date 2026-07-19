// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package toml_test

import (
	"bytes"
	_ "embed"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/toml"
)

//go:embed testdata/expected.toml
var expected []byte

func TestMerge(t *testing.T) {
	t.Parallel()

	out, err := toml.Merge(map[string]toml.Part{
		"testdata/1.toml": {Contents: mustRead(t, "testdata/1.toml"), Origin: "testdata/1.toml"},
		"testdata/2.toml": {Contents: mustRead(t, "testdata/2.toml"), Origin: "testdata/2.toml"},
		"testdata/3.toml": {Contents: mustRead(t, "testdata/3.toml"), Origin: "testdata/3.toml"},
	})
	require.NoError(t, err)

	assert.Equal(t, string(expected), string(out))
}

func TestMergeOrder(t *testing.T) {
	t.Parallel()

	out, err := toml.Merge(map[string]toml.Part{
		"20-second.part": {Contents: []byte("[metrics]\naddress = 'second'\n"), Origin: "in-memory first alphabetically"},
		"10-first.part":  {Contents: []byte("version = 2\n[metrics]\naddress = 'first'\n"), Origin: "file last alphabetically"},
	})
	require.NoError(t, err)

	firstIndex := bytes.Index(out, []byte("## file last alphabetically"))
	secondIndex := bytes.Index(out, []byte("## in-memory first alphabetically"))

	require.NotEqual(t, -1, firstIndex)
	require.NotEqual(t, -1, secondIndex)
	assert.Less(t, firstIndex, secondIndex)
	assert.Contains(t, string(out), "address = 'second'")
}

func TestMergeInvalid(t *testing.T) {
	t.Parallel()

	_, err := toml.Merge(map[string]toml.Part{"invalid.part": {Contents: []byte("[invalid"), Origin: "in-memory invalid"}})

	assert.ErrorContains(t, err, `error decoding "invalid.part"`)
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)

	return contents
}
