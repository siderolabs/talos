// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package xfs_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

func TestUnixRootConfined(t *testing.T) {
	t.Parallel()

	if uid := os.Getuid(); uid != 0 {
		t.Skipf("skipping test, not running as root (uid %d)", uid)
	}

	root := &xfs.UnixRoot{FS: fsopen.New("tmpfs")}
	require.NoError(t, root.OpenFS())

	t.Cleanup(func() { root.Close() }) //nolint:errcheck

	fd, err := root.Fd()
	require.NoError(t, err)

	require.NoError(t, xfs.MkdirAll(root, "dir", 0o755))
	require.NoError(t, xfs.WriteFile(root, "dir/file", []byte("inside"), 0o644))

	// the remove subtest gets its own symlinks, as parallel subtests run in any order
	for name, target := range map[string]string{
		"abs":       os.TempDir(),
		"up":        "../..",
		"inside":    "dir",
		"rm-abs":    os.TempDir(),
		"rm-inside": "dir",
	} {
		require.NoError(t, unix.Symlinkat(target, fd, name))
	}

	contents, err := xfs.ReadFile(root, "inside/file")
	require.NoError(t, err)
	assert.Equal(t, "inside", string(contents))

	for _, name := range []string{"abs/escape", "up/escape", "../escape"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, xfs.WriteFile(root, name, []byte("escape"), 0o644), unix.EXDEV)
			assert.ErrorIs(t, root.Mkdir(name, 0o755), unix.EXDEV)
			assert.ErrorIs(t, root.Remove(name), unix.EXDEV)
			assert.ErrorIs(t, root.Rename("dir/file", name), unix.EXDEV)
		})
	}

	for _, name := range []string{"dir/../dir/file", "dir/./file", "dir/"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := xfs.ReadFile(root, name)
			assert.ErrorIs(t, err, unix.EINVAL)
			assert.ErrorIs(t, root.Remove(name), unix.EINVAL)
		})
	}

	t.Run("remove symlink", func(t *testing.T) {
		t.Parallel()

		// a symlink is removed itself, its target stays
		require.NoError(t, root.Remove("rm-abs"))
		require.NoError(t, xfs.RemoveAll(root, "rm-inside"))

		contents, err := xfs.ReadFile(root, "dir/file")
		require.NoError(t, err)
		assert.Equal(t, "inside", string(contents))
	})
}
