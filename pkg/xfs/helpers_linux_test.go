// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package xfs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/xfs"
)

func TestAsOSFile(t *testing.T) {
	t.Parallel()

	root := &xfs.OSRoot{Shadow: t.TempDir()}
	require.NoError(t, root.OpenFS())

	t.Cleanup(func() {
		require.NoError(t, root.Close())
	})

	writeFile(t, root, "file", testFileContent)

	f, err := xfs.Open(root, "file")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	osf, err := xfs.AsOSFile(f, "file")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, osf.Close())
	})

	assertCloseOnExec(t, osf.Fd())
}
