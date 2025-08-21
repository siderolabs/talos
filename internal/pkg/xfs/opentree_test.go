// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build unix

package xfs_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

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
			err := fs.Close()
			require.NoError(t, err)
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
			err := fs.UnmountFrom(roRoot.Shadow)
			require.NoError(t, err)
		})

		bfs := opentree.NewFromPath(roRoot.Shadow)

		rwRoot := &xfs.UnixRoot{FS: bfs}

		err = rwRoot.OpenFS()
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fs.Close()
			require.NoError(t, err)
		})

		testFilesystem(t, rwRoot, roRoot)
	})

	t.Run("FileDescriptor", func(t *testing.T) {
		t.Parallel()

		if !hasKernel(t, "6.15.0") {
			t.Skip("OpenTree on Anonymous FS requires kernel 6.15.0+")
		}

		fs := fsopen.New("tmpfs")

		roRoot := &xfs.UnixRoot{FS: fs}

		err := roRoot.OpenFS()
		require.NoError(t, err)

		fd, err := roRoot.Fd()
		require.NoError(t, err)

		bfs := opentree.NewFromFd(fd)

		rwRoot := &xfs.UnixRoot{FS: bfs}

		err = rwRoot.OpenFS()
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fs.Close()
			require.NoError(t, err)
		})

		testFilesystem(t, rwRoot, roRoot)
	})
}

func hasKernel(tb testing.TB, kernelVersion string) bool {
	tb.Helper()

	capabilityAt, err := semver.Parse(kernelVersion)
	require.NoError(tb, err)

	buf := new(unix.Utsname)

	err = unix.Uname(buf)
	require.NoError(tb, err)

	current, err := semver.Parse(string(bytes.TrimRight(buf.Release[:], "\x00")))
	require.NoError(tb, err)

	return current.GE(capabilityAt)
}
