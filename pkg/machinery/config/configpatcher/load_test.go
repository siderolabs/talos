// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher_test

import (
	_ "embed"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
)

//go:embed testdata/patch.json
var jsonPatch []byte

//go:embed testdata/patch.yaml
var yamlPatch []byte

//go:embed testdata/strategic.yaml
var strategicPatch []byte

func TestLoadJSON(t *testing.T) {
	raw, err := configpatcher.LoadPatch(jsonPatch)
	require.NoError(t, err)

	p, ok := raw.(jsonpatch.Patch)
	require.True(t, ok)

	assert.Len(t, p, 1)
	assert.Equal(t, p[0].Kind(), "add")

	var path string

	path, err = p[0].Path()

	require.NoError(t, err)
	assert.Equal(t, path, "/machine/certSANs")
}

func TestLoadYAML(t *testing.T) {
	raw, err := configpatcher.LoadPatch(yamlPatch)
	require.NoError(t, err)

	p, ok := raw.(jsonpatch.Patch)
	require.True(t, ok)

	assert.Len(t, p, 1)
	assert.Equal(t, p[0].Kind(), "add")

	var path string

	path, err = p[0].Path()

	require.NoError(t, err)
	assert.Equal(t, path, "/some/path")

	var v any

	v, err = p[0].ValueInterface()
	require.NoError(t, err)
	assert.Equal(t, v, []any{"a", "b", "c"})
}

func TestLoadStrategic(t *testing.T) {
	raw, err := configpatcher.LoadPatch(strategicPatch)
	require.NoError(t, err)

	p, ok := raw.(configpatcher.StrategicMergePatch)
	require.True(t, ok)

	assert.Equal(t, "foo.bar", p.Provider().NetworkHostnameConfig().Hostname())
}

func TestLoadJSONPatches(t *testing.T) {
	patchList, err := configpatcher.LoadPatches([]string{
		"@testdata/patch.json",
		"@testdata/patch.yaml",
		`[{"op":"replace","path":"/some","value": []}]`,
	})
	require.NoError(t, err)

	require.Len(t, patchList, 1)

	raw := patchList[0]

	p, ok := raw.(jsonpatch.Patch)
	require.True(t, ok)

	assert.Len(t, p, 3)
	assert.Equal(t, p[0].Kind(), "add")
	assert.Equal(t, p[1].Kind(), "add")
	assert.Equal(t, p[2].Kind(), "replace")
}

func TestLoadMixedPatches(t *testing.T) {
	patchList, err := configpatcher.LoadPatches([]string{
		"@testdata/patch.json",
		"@testdata/strategic.yaml",
		"@testdata/patch.yaml",
		`[{"op":"replace","path":"/some","value": []}]`,
	})
	require.NoError(t, err)

	require.Len(t, patchList, 3)

	assert.IsType(t, jsonpatch.Patch{}, patchList[0])
	assert.Implements(t, (*configpatcher.StrategicMergePatch)(nil), patchList[1])
	assert.IsType(t, jsonpatch.Patch{}, patchList[2])
}
