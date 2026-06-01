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

	"github.com/siderolabs/talos/pkg/xfs"
)

const testDir = "testdir"

var testFileContent = []byte("test content")

func testFilesystem(t *testing.T, rwRoot xfs.Root, roRoot xfs.Root) {
	if roRoot == nil {
		roRoot = rwRoot
	}

	touchTree(t, rwRoot, filepath.Join(testDir, "root.test"))

	t.Run("Open", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "open.test")

		touchTree(t, rwRoot, name)

		actual, err := xfs.Open(rwRoot, name)
		assert.NoError(t, err)
		assert.NoError(t, actual.Close())
	})

	t.Run("OpenFile", func(t *testing.T) {
		t.Parallel()

		flags := os.O_RDWR | os.O_CREATE
		name := filepath.Join(testDir, "open-file.test")

		actual, err := xfs.OpenFile(rwRoot, name, flags, 0o644)
		require.NoError(t, err)
		assert.NoError(t, actual.Close())
	})

	t.Run("WriteFile", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "write-file.test")

		err := xfs.WriteFile(rwRoot, name, testFileContent, 0o644)
		assert.NoError(t, err)
	})

	t.Run("ReadFile", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "read-file.test")

		writeFile(t, rwRoot, name, testFileContent)

		actual, err := xfs.ReadFile(rwRoot, name)
		assert.NoError(t, err)
		assert.Equal(t, testFileContent, actual)
	})

	t.Run("Mkdir", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "mkdir.test")

		err := xfs.Mkdir(rwRoot, name, 0o755)
		assert.NoError(t, err)

		testIsDir(t, roRoot, name)
	})

	t.Run("MkdirAll", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "mkdir-all.d", "test.d")

		err := xfs.MkdirAll(rwRoot, name, 0o755)
		assert.NoError(t, err)

		components := xfs.SplitPath(name)

		for i := range len(components) + 1 {
			dir := filepath.Join(components[:i]...)
			if dir == "" {
				// empty name, continue...
				continue
			}

			testIsDir(t, roRoot, dir)
		}
	})

	t.Run("MkdirTemp", func(t *testing.T) {
		t.Parallel()

		name, err := xfs.MkdirTemp(rwRoot, testDir, "")
		assert.NoError(t, err)

		components := xfs.SplitPath(name)

		for i := range len(components) + 1 {
			dir := filepath.Join(components[:i]...)
			if dir == "" {
				// empty name, continue...
				continue
			}

			testIsDir(t, roRoot, dir)
		}
	})

	t.Run("Remove", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "remove.test")

		touchTree(t, rwRoot, name)

		err := xfs.Remove(rwRoot, name)
		require.NoError(t, err)

		_, err = xfs.Stat(roRoot, name)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("RemoveAll", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "remove-all.d", "file.test")

		touchTree(t, rwRoot, name)

		components := xfs.SplitPath(name)

		err := xfs.RemoveAll(rwRoot, filepath.Join(components[:2]...))
		require.NoError(t, err)

		for i := range len(components[1:]) {
			test := filepath.Join(components[:len(components)-i]...)
			if test == "" {
				// empty name, continue...
				continue
			}

			_, err := xfs.Stat(roRoot, test)
			require.ErrorIs(t, err, os.ErrNotExist, "stat %q should not exist", test)
		}
	})

	t.Run("Stat", func(t *testing.T) {
		t.Parallel()

		name := filepath.Join(testDir, "stat.d", "stat.test")

		touchTree(t, rwRoot, name)

		components := xfs.SplitPath(name)

		for i := range components {
			dir := filepath.Join(components[:i]...)
			if dir == "" {
				// empty name, continue...
				continue
			}

			actual, err := xfs.Stat(roRoot, dir)
			require.NoError(t, err, "stat dir %q failed", dir)
			assert.True(t, actual.IsDir())
		}

		actual, err := xfs.Stat(roRoot, name)
		require.NoError(t, err, "stat file %q failed", name)
		assert.False(t, actual.IsDir())
	})

	t.Run("Rename", func(t *testing.T) {
		t.Parallel()

		t.Run("Dir", func(t *testing.T) {
			t.Parallel()

			oldName := filepath.Join(testDir, "rename.old.d", "test")
			newName := filepath.Join(testDir, "rename.new.d", "test")

			touchTree(t, rwRoot, oldName)

			err := xfs.Rename(rwRoot, filepath.Dir(oldName), filepath.Dir(newName))
			assert.NoError(t, err)

			newDirStat, err := xfs.Stat(roRoot, filepath.Dir(newName))
			require.NoError(t, err, "stat dir %q failed", filepath.Dir(newName))
			assert.True(t, newDirStat.IsDir())

			_, err = xfs.Stat(roRoot, newName)
			require.NoError(t, err, "stat file %q failed", newName)

			_, err = xfs.Stat(roRoot, oldName)
			require.ErrorIs(t, err, os.ErrNotExist, "stat dir %q failed", filepath.Dir(oldName))
		})

		t.Run("File", func(t *testing.T) {
			t.Parallel()

			oldName := filepath.Join(testDir, "rename.old.test")
			newName := filepath.Join(testDir, "rename.new.test")

			touchTree(t, rwRoot, oldName)

			err := xfs.Rename(rwRoot, oldName, newName)
			assert.NoError(t, err)

			_, err = xfs.Stat(roRoot, newName)
			require.NoError(t, err, "stat file %q failed", newName)

			_, err = xfs.Stat(roRoot, oldName)
			require.ErrorIs(t, err, os.ErrNotExist, "stat file %q failed", oldName)
		})
	})
}

func writeFile(tb testing.TB, root xfs.Root, name string, content []byte) {
	tb.Helper()

	err := xfs.WriteFile(root, name, content, 0o644)
	require.NoError(tb, err)
}

func touchTree(tb testing.TB, root xfs.Root, tree string) {
	tb.Helper()

	components := xfs.SplitPath(tree)

	for i := range components {
		dir := filepath.Join(components[:i]...)
		if dir == "" {
			// empty name, continue...
			continue
		}

		err := root.Mkdir(dir, 0o755)
		if os.IsExist(err) {
			continue
		}

		require.NoError(tb, err, "creating dir %q failed", dir)
	}

	f, err := root.OpenFile(tree, os.O_CREATE|os.O_TRUNC, 0o644)
	require.NoError(tb, err, "creating tree %q failed", tree)

	tb.Cleanup(func() {
		assert.NoError(tb, f.Close(), "closing file %q failed", tree)
	})
}

func testIsDir(tb testing.TB, root xfs.Root, name string) {
	actual, err := root.Open(name)
	require.NoError(tb, err, "opening %q failed", name)

	tb.Cleanup(func() {
		assert.NoError(tb, actual.Close(), "closing %q failed", name)
	})

	info, err := actual.Stat()
	require.NoError(tb, err, "stat on %q failed", name)
	assert.True(tb, info.IsDir())
}
