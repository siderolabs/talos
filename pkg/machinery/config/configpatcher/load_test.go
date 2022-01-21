// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/config/configpatcher"
)

//go:embed testdata/patch.json
var jsonPatch []byte

//go:embed testdata/patch.yaml
var yamlPatch []byte

func TestLoadJSON(t *testing.T) {
	p, err := configpatcher.LoadPatch(jsonPatch)
	require.NoError(t, err)

	assert.Len(t, p, 1)
	assert.Equal(t, p[0].Kind(), "add")

	var path string
	path, err = p[0].Path()

	require.NoError(t, err)
	assert.Equal(t, path, "/machine/certSANs")
}

func TestLoadYAML(t *testing.T) {
	p, err := configpatcher.LoadPatch(yamlPatch)
	require.NoError(t, err)

	assert.Len(t, p, 1)
	assert.Equal(t, p[0].Kind(), "add")

	var path string
	path, err = p[0].Path()

	require.NoError(t, err)
	assert.Equal(t, path, "/some/path")

	var v interface{}
	v, err = p[0].ValueInterface()
	require.NoError(t, err)
	assert.Equal(t, v, []interface{}{"a", "b", "c"})
}

func TestLoadPatches(t *testing.T) {
	p, err := configpatcher.LoadPatches([]string{
		"@testdata/patch.json",
		"@testdata/patch.yaml",
		`[{"op":"replace","path":"/some","value": []}]`,
	})
	require.NoError(t, err)

	assert.Len(t, p, 3)
	assert.Equal(t, p[0].Kind(), "add")
	assert.Equal(t, p[1].Kind(), "add")
	assert.Equal(t, p[2].Kind(), "replace")
}
