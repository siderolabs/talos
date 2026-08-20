// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher_test

import (
	"bytes"
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"os"
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

	require.Len(t, p.Documents(), 1)
	assert.Equal(t, "v1alpha1", p.Documents()[0].Kind())

	b, err := p.Bytes()
	require.NoError(t, err)

	assert.Equal(t, strategicPatch, b)
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

func TestLoadStraightFilename(t *testing.T) {
	patchList, err := configpatcher.LoadPatches([]string{
		"testdata/strategic.yaml",
		`[{"op":"replace","path":"/some","value": []}]`,
	})
	require.NoError(t, err)

	require.Len(t, patchList, 2)

	assert.Implements(t, (*configpatcher.StrategicMergePatch)(nil), patchList[0])
	assert.IsType(t, jsonpatch.Patch{}, patchList[1])
}

// remotePatchSizeLimit mirrors the package's own cap on a remote patch response; it is
// duplicated here because the test lives outside the package.
const remotePatchSizeLimit = 1 << 20

// remotePatchServer serves the same testdata the local-file tests use, over http.
func remotePatchServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	for path, contents := range map[string][]byte{
		"/patch.json":     jsonPatch,
		"/patch.yaml":     yamlPatch,
		"/strategic.yaml": strategicPatch,
	} {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			w.Write(contents) //nolint:errcheck
		})
	}

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

func TestLoadRemotePatches(t *testing.T) {
	server := remotePatchServer(t)

	patchList, err := configpatcher.LoadPatches([]string{
		server.URL + "/patch.json",
		"@" + server.URL + "/strategic.yaml",
		server.URL + "/patch.yaml",
		`[{"op":"replace","path":"/some","value": []}]`,
	})
	require.NoError(t, err)

	require.Len(t, patchList, 3)

	assert.IsType(t, jsonpatch.Patch{}, patchList[0])
	assert.Implements(t, (*configpatcher.StrategicMergePatch)(nil), patchList[1])

	merged, ok := patchList[2].(jsonpatch.Patch)
	require.True(t, ok)
	assert.Len(t, merged, 2)
}

func TestLoadRemoteAndLocalPatches(t *testing.T) {
	server := remotePatchServer(t)

	patchList, err := configpatcher.LoadPatches([]string{
		"@testdata/patch.json",
		server.URL + "/patch.yaml",
	})
	require.NoError(t, err)

	// both are JSON patches, so they are merged into one exactly as two local ones are
	require.Len(t, patchList, 1)

	merged, ok := patchList[0].(jsonpatch.Patch)
	require.True(t, ok)
	assert.Len(t, merged, 2)
}

func TestLoadRemotePatchStatus(t *testing.T) {
	server := remotePatchServer(t)

	_, err := configpatcher.LoadPatches([]string{server.URL + "/no-such-patch.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestLoadRemotePatchAtSizeLimit(t *testing.T) {
	// a patch exactly at the limit is accepted, and is not truncated: a truncated
	// strategic merge patch would either fail to parse or, worse, apply partially
	padded := append(append([]byte{}, strategicPatch...), '\n')
	padded = append(padded, bytes.Repeat([]byte("#"), remotePatchSizeLimit-len(padded))...)
	require.Len(t, padded, remotePatchSizeLimit)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(padded) //nolint:errcheck
	}))
	t.Cleanup(server.Close)

	patchList, err := configpatcher.LoadPatches([]string{server.URL + "/strategic.yaml"})
	require.NoError(t, err)

	require.Len(t, patchList, 1)
	assert.Implements(t, (*configpatcher.StrategicMergePatch)(nil), patchList[0])
}

func TestLoadRemotePatchOverSizeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(bytes.Repeat([]byte("#"), remotePatchSizeLimit+1)) //nolint:errcheck
	}))
	t.Cleanup(server.Close)

	_, err := configpatcher.LoadPatches([]string{server.URL + "/strategic.yaml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestLoadRemotePatchContext(t *testing.T) {
	server := remotePatchServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := configpatcher.LoadPatchesWithContext(ctx, []string{server.URL + "/patch.json"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestLoadUnknownSchemeIsAFilename(t *testing.T) {
	// only http and https are fetched; anything else keeps its previous meaning, which
	// for a single word with no spaces is "a filename"
	_, err := configpatcher.LoadPatches([]string{"file://testdata/strategic.yaml"})
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
