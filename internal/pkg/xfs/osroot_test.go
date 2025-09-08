// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package xfs_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/xfs"
)

func TestOs(t *testing.T) {
	t.Parallel()

	t.Run("OSRoot", func(t *testing.T) {
		t.Parallel()

		root := &xfs.OSRoot{Shadow: t.TempDir()}

		require.NoError(t, root.OpenFS())

		t.Cleanup(func() {
			require.NoError(t, root.Close())
		})

		testFilesystem(t, root, nil)
	})
}
