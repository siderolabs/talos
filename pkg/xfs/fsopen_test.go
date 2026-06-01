// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package xfs_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

func TestFsopen(t *testing.T) {
	t.Parallel()

	if uid := os.Getuid(); uid != 0 {
		t.Skipf("skipping test, not running as root (uid %d)", uid)
	}

	for _, tc := range []struct {
		fstype string
		opts   []fsopen.Option
	}{
		{fstype: "tmpfs"},
	} {
		t.Run(tc.fstype, func(t *testing.T) {
			t.Parallel()

			root := &xfs.UnixRoot{FS: fsopen.New(tc.fstype, tc.opts...)}

			err := root.OpenFS()
			require.NoError(t, err)

			t.Cleanup(func() {
				require.NoError(t, root.Close())
			})

			testFilesystem(t, root, nil)
		})
	}
}
