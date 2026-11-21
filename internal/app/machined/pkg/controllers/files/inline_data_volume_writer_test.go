// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
)

func listDir(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))

	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	slices.Sort(names)

	return names
}

func assertFile(t *testing.T, dir, path, contents string, mode fs.FileMode) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, path))
	require.NoError(t, err)
	assert.Equal(t, contents, string(data))

	st, err := os.Stat(filepath.Join(dir, path))
	require.NoError(t, err)
	assert.Equal(t, mode, st.Mode().Perm(), path)
}

func writeInlineDataVolume(t *testing.T, dir string, files []configconfig.InlineDataVolumeFile) string {
	t.Helper()

	root, err := os.OpenRoot(dir)
	require.NoError(t, err)

	t.Cleanup(func() { root.Close() }) //nolint:errcheck

	hash, err := filesctrl.WriteInlineDataVolume(root, files)
	require.NoError(t, err)

	assert.Equal(t, filesctrl.InlineDataVolumeContentsHash(files), hash)

	return hash
}

func TestWriteInlineDataVolume(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	v1 := []configconfig.InlineDataVolumeFile{
		{Path: "a.conf", Mode: 0o644, Contents: "a1"},
		{Path: "sub/dir/b.ini", Mode: 0o600, Contents: "b1"},
		{Path: "z.yaml", Mode: 0o400, Contents: "z1"},
	}

	hash1 := writeInlineDataVolume(t, dir, v1)

	assert.Equal(t, []string{"..data", "..data-" + hash1, "a.conf", "sub", "z.yaml"}, listDir(t, dir))

	link, err := os.Readlink(filepath.Join(dir, "..data"))
	require.NoError(t, err)
	assert.Equal(t, "..data-"+hash1, link)

	link, err = os.Readlink(filepath.Join(dir, "sub"))
	require.NoError(t, err)
	assert.Equal(t, "..data/sub", link)

	assertFile(t, dir, "a.conf", "a1", 0o644)
	assertFile(t, dir, "sub/dir/b.ini", "b1", 0o600)
	assertFile(t, dir, "z.yaml", "z1", 0o400)

	assert.Equal(t, hash1, writeInlineDataVolume(t, dir, v1))
	assert.Equal(t, []string{"..data", "..data-" + hash1, "a.conf", "sub", "z.yaml"}, listDir(t, dir))

	v2 := []configconfig.InlineDataVolumeFile{
		{Path: "a.conf", Mode: 0o640, Contents: "a2"},
		{Path: "new.txt", Mode: 0o644, Contents: "n"},
		{Path: "sub/dir/b.ini", Mode: 0o600, Contents: "b1"},
	}

	hash2 := writeInlineDataVolume(t, dir, v2)
	assert.NotEqual(t, hash1, hash2)

	assert.Equal(t, []string{"..data", "..data-" + hash2, "a.conf", "new.txt", "sub"}, listDir(t, dir))

	assertFile(t, dir, "a.conf", "a2", 0o640)
	assertFile(t, dir, "new.txt", "n", 0o644)
	assertFile(t, dir, "sub/dir/b.ini", "b1", 0o600)

	v3 := slices.Clone(v2)
	v3[0].Mode = 0o644

	assert.NotEqual(t, hash2, writeInlineDataVolume(t, dir, v3))
	assertFile(t, dir, "a.conf", "a2", 0o644)
}

func TestWriteInlineDataVolumeRepairs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	files := []configconfig.InlineDataVolumeFile{
		{Path: "a.conf", Mode: 0o644, Contents: "a"},
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "stray"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "..data-"+filesctrl.InlineDataVolumeContentsHash(files), "garbage"), 0o755))
	require.NoError(t, os.Symlink("nowhere", filepath.Join(dir, "..data_tmp")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "a.conf"), 0o755))

	hash := writeInlineDataVolume(t, dir, files)

	assert.Equal(t, []string{"..data", "..data-" + hash, "a.conf"}, listDir(t, dir))
	assert.Equal(t, []string{"a.conf"}, listDir(t, filepath.Join(dir, "..data-"+hash)))
	assertFile(t, dir, "a.conf", "a", 0o644)

	require.NoError(t, os.Remove(filepath.Join(dir, "a.conf")))

	writeInlineDataVolume(t, dir, files)
	assertFile(t, dir, "a.conf", "a", 0o644)
}
