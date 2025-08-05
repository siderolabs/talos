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

func TestSubfs(t *testing.T) {
	t.Parallel()

	if uid := os.Getuid(); uid != 0 {
		t.Skipf("skipping test, not running as root (uid %d)", uid)
	}

	t.Run("TempDir", func(t *testing.T) {
		t.Parallel()

		testRoot := t.TempDir()

		fsc := opentree.NewFromPath(testRoot)

		fs, err := xfs.NewUnix(fsc)
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fsc.Close()
			require.NoError(t, err)
		})

		testFilesystem(t, fs, fs)
	})

	t.Run("MountDir", func(t *testing.T) {
		t.Parallel()

		fsc, err := fsopen.New("tmpfs")
		require.NoError(t, err)

		fs, err := xfs.NewUnix(fsc)
		require.NoError(t, err)

		fs.Shadow, err = fsc.MountAt(t.TempDir())
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fsc.UnmountFrom(fs.Shadow)
			require.NoError(t, err)
		})

		sfsc := opentree.NewFromPath(fs.MountPoint())

		sfs, err := xfs.NewUnix(sfsc)
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fsc.Close()
			require.NoError(t, err)
		})

		testFilesystem(t, sfs, fs)
	})

	t.Run("FileDescriptor", func(t *testing.T) {
		t.Parallel()

		if !hasKernel(t, "6.15.0") {
			t.Skip("OpenTree on Anonymous FS requires kernel 6.15.0+")
		}

		fsc, err := fsopen.New("tmpfs")
		require.NoError(t, err)

		fs, err := xfs.NewUnix(fsc)
		require.NoError(t, err)

		sfsc := opentree.NewFromFd(fs.FileDescriptor())

		sfs, err := xfs.NewUnix(sfsc)
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fsc.Close()
			require.NoError(t, err)
		})

		testFilesystem(t, sfs, fs)
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
