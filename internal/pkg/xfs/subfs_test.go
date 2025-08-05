// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build unix

package xfs_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/xfs"
	"github.com/siderolabs/talos/internal/pkg/xfs/anonfs"
	"github.com/siderolabs/talos/internal/pkg/xfs/subfs"
)

func TestSubfs(t *testing.T) {
	t.Parallel()

	if uid := os.Getuid(); uid != 0 {
		t.Skipf("skipping test, not running as root (uid %d)", uid)
	}

	t.Run("TempDir", func(t *testing.T) {
		t.Parallel()

		testRoot := t.TempDir()

		fsc := subfs.NewFrom(testRoot)

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

		fsc, err := anonfs.New(anonfs.TypeTmpfs)
		require.NoError(t, err)

		fs, err := xfs.NewUnix(fsc)
		require.NoError(t, err)

		fs.Shadow, err = fsc.MountAt(t.TempDir())
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fsc.UnmountFrom(fs.Shadow)
			require.NoError(t, err)
		})

		sfsc := subfs.NewFrom(fs.MountPoint())

		sfs, err := xfs.NewUnix(sfsc)
		require.NoError(t, err)

		t.Cleanup(func() {
			err := fsc.Close()
			require.NoError(t, err)
		})

		testFilesystem(t, sfs, fs)
	})
}
