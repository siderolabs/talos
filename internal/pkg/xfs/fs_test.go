// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package xfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/xfs"
)

const testDir = "testdir"

var testFileContent = []byte("test content")

func testFilesystem(t *testing.T, rwfs xfs.FS, rofs xfs.FS) {
	if rofs == nil {
		rofs = rwfs
	}

	touchTree(t, rwfs, filepath.Join(testDir, "root.test"))

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "open.test")

		touchTree(t, rwfs, name)

		actual, err := xfs.Open(rwfs, name)
		assert.NoError(t, err)
		assert.NoError(t, actual.Close())
	})

	t.Run("OpenFile", func(t *testing.T) {
		t.Parallel()

		flags := os.O_RDWR | os.O_CREATE
		name := filepath.Join(testDir, "open-file.test")

		actual, err := xfs.OpenFile(rwfs, name, flags, 0o644)
		require.NoError(t, err)
		assert.NoError(t, actual.Close())
	})

	t.Run("WriteFile", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "write-file.test")

		err := xfs.WriteFile(rwfs, name, testFileContent, 0o644)
		assert.NoError(t, err)
	})

	t.Run("ReadFile", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "read-file.test")

		writeFile(t, rwfs, name, testFileContent)

		actual, err := xfs.ReadFile(rwfs, name)
		assert.NoError(t, err)
		assert.Equal(t, testFileContent, actual)
	})

	t.Run("Mkdir", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "mkdir.test")

		err := xfs.Mkdir(rwfs, name, 0o755)
		assert.NoError(t, err)

		testIsDir(t, rofs, name)
	})

	t.Run("MkdirAll", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "mkdir-all.d", "test.d")

		err := xfs.MkdirAll(rwfs, name, 0o755)
		assert.NoError(t, err)

		components := xfs.SplitPath(name)

		for i := range len(components) + 1 {
			dir := filepath.Join(components[:i]...)
			if dir == "" {
				// empty name, continue...
				continue
			}

			testIsDir(t, rofs, dir)
		}
	})

	t.Run("MkdirTemp", func(t *testing.T) {
		t.Parallel()

		name, err := xfs.MkdirTemp(rwfs, testDir, "")
		assert.NoError(t, err)

		components := xfs.SplitPath(name)

		for i := range len(components) + 1 {
			dir := filepath.Join(components[:i]...)
			if dir == "" {
				// empty name, continue...
				continue
			}

			testIsDir(t, rofs, dir)
		}
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "remove.test")

		touchTree(t, rwfs, name)

		err := xfs.Remove(rwfs, name)
		require.NoError(t, err)

		_, err = xfs.Stat(rofs, name)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("RemoveAll", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "remove-all.d", "file.test")

		touchTree(t, rwfs, name)

		components := xfs.SplitPath(name)

		err := xfs.RemoveAll(rwfs, filepath.Join(components[:2]...))
		require.NoError(t, err)

		for i := range len(components[1:]) {
			test := filepath.Join(components[:len(components)-i]...)
			if test == "" {
				// empty name, continue...
				continue
			}

			_, err := xfs.Stat(rofs, test)
			require.ErrorIs(t, err, os.ErrNotExist, "stat %q should not exist", test)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "stat.d/stat.test")

		touchTree(t, rwfs, name)

		components := xfs.SplitPath(name)

		for i := range components {
			dir := filepath.Join(components[:i]...)
			if dir == "" {
				// empty name, continue...
				continue
			}

			actual, err := xfs.Stat(rofs, dir)
			require.NoError(t, err, "stat dir %q failed", dir)
			assert.True(t, actual.IsDir())
		}

		actual, err := xfs.Stat(rofs, name)
		require.NoError(t, err, "stat file %q failed", name)
		assert.False(t, actual.IsDir())
	})
}

func writeFile(tb testing.TB, fs xfs.FS, name string, content []byte) {
	tb.Helper()

	err := xfs.WriteFile(fs, name, content, 0o644)
	require.NoError(tb, err)
}

func touchTree(tb testing.TB, fs xfs.FS, tree string) {
	tb.Helper()

	components := xfs.SplitPath(tree)

	for i := range components {
		dir := filepath.Join(components[:i]...)
		if dir == "" {
			// empty name, continue...
			continue
		}

		err := fs.Mkdir(dir, 0o755)
		if os.IsExist(err) {
			continue
		}

		require.NoError(tb, err)
	}

	f, err := fs.OpenFile(tree, os.O_CREATE|os.O_TRUNC, 0o644)
	require.NoError(tb, err, "creating tree %q failed", tree)

	tb.Cleanup(func() {
		assert.NoError(tb, f.Close(), "closing file %q failed", tree)
	})
}

func testIsDir(tb testing.TB, fs xfs.FS, name string) {
	actual, err := fs.Open(name)
	require.NoError(tb, err, "opening %q failed", name)

	tb.Cleanup(func() {
		assert.NoError(tb, actual.Close(), "closing %q failed", name)
	})

	info, err := actual.Stat()
	require.NoError(tb, err, "stat on %q failed", name)
	assert.True(tb, info.IsDir())
}
