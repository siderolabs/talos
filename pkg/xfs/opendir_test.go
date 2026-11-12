// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package xfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/opendir"
)

func TestOpendir(t *testing.T) {
	t.Parallel()

	t.Run("TempDir", func(t *testing.T) {
		t.Parallel()

		root := &xfs.UnixRoot{FS: opendir.NewFromPath(t.TempDir())}
		require.NoError(t, root.OpenFS())

		t.Cleanup(func() {
			require.NoError(t, root.Close())
		})

		testFilesystem(t, root, root)
	})

	t.Run("Beneath", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()

		require.NoError(t, os.Mkdir(filepath.Join(parent, "dir"), 0o755))
		require.NoError(t, os.Symlink(os.TempDir(), filepath.Join(parent, "abs")))

		dirfd, err := unix.Open(parent, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
		require.NoError(t, err)

		t.Cleanup(func() { unix.Close(dirfd) }) //nolint:errcheck

		for _, name := range []string{"..", "abs", "dir/../.."} {
			_, err = opendir.NewBeneath(dirfd, name).Open()
			assert.ErrorIs(t, err, unix.EXDEV, name)
		}

		root := &xfs.UnixRoot{FS: opendir.NewBeneath(dirfd, "dir")}
		require.NoError(t, root.OpenFS())

		t.Cleanup(func() {
			require.NoError(t, root.Close())
		})

		require.NoError(t, xfs.WriteFile(root, "file", []byte("inside"), 0o644))
		assert.ErrorIs(t, xfs.WriteFile(root, "../escape", []byte("escape"), 0o644), unix.EXDEV)

		contents, err := os.ReadFile(filepath.Join(parent, "dir", "file"))
		require.NoError(t, err)
		assert.Equal(t, "inside", string(contents))
		assert.NoFileExists(t, filepath.Join(parent, "escape"))
	})
}
