// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build unix

package xfs_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/xfs"
	"github.com/siderolabs/talos/internal/pkg/xfs/fsopen"
	"github.com/siderolabs/talos/internal/pkg/xfs/opentree"
)

func TestOpentree(t *testing.T) {
	t.Parallel()

	if uid := os.Getuid(); uid != 0 {
		t.Skipf("skipping test, not running as root (uid %d)", uid)
	}

	t.Run("TempDir", func(t *testing.T) {
		t.Parallel()

		testRoot := t.TempDir()

		fs := opentree.NewFromPath(testRoot)

		root := &xfs.UnixRoot{FS: fs}

		err := root.OpenFS()
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, root.Close())
		})

		testFilesystem(t, root, root)
	})

	t.Run("MountDir", func(t *testing.T) {
		t.Parallel()

		fs := fsopen.New("tmpfs")

		roRoot := &xfs.UnixRoot{FS: fs}

		err := roRoot.OpenFS()
		require.NoError(t, err)

		roRoot.Shadow, err = fs.MountAt(t.TempDir())
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, roRoot.Close())
			require.NoError(t, fs.UnmountFrom(roRoot.Shadow))
		})

		bfs := opentree.NewFromPath(roRoot.Shadow)

		rwRoot := &xfs.UnixRoot{FS: bfs}

		err = rwRoot.OpenFS()
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, rwRoot.Close())
		})

		testFilesystem(t, rwRoot, roRoot)
	})

	t.Run("FileDescriptor", func(t *testing.T) {
		t.Parallel()

		ok, err := runtime.KernelCapabilities().OpentreeOnAnonymousFS()
		require.NoError(t, err)

		if !ok {
			t.Skip("OpenTree on Anonymous FS requires kernel 6.15.0+")
		}

		fs := fsopen.New("tmpfs")

		roRoot := &xfs.UnixRoot{FS: fs}

		err = roRoot.OpenFS()
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, roRoot.Close())
		})

		fd, err := roRoot.Fd()
		require.NoError(t, err)

		bfs := opentree.NewFromFd(fd)

		rwRoot := &xfs.UnixRoot{FS: bfs}

		err = rwRoot.OpenFS()
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, fs.Close())
		})

		testFilesystem(t, rwRoot, roRoot)
	})
}
