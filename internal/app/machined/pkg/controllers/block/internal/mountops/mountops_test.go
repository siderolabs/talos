// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mountops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/mountops"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestCreateMountPoint(t *testing.T) {
	t.Parallel()

	mountSpec := block.MountSpec{
		FileMode: 0o710,
		UID:      os.Getuid(),
		GID:      os.Getgid(),
	}

	t.Run("new", func(t *testing.T) {
		t.Parallel()

		target := filepath.Join(t.TempDir(), "parent", "vol")

		created, err := mountops.CreateMountPoint(target, mountSpec)
		require.NoError(t, err)
		assert.True(t, created)

		st, err := os.Stat(target)
		require.NoError(t, err)
		assert.True(t, st.IsDir())
		assert.Equal(t, os.FileMode(0o710), st.Mode().Perm())

		mountops.RemoveMountPoint(zaptest.NewLogger(t), target)

		assert.NoDirExists(t, target)
		assert.DirExists(t, filepath.Dir(target))
	})

	t.Run("existing", func(t *testing.T) {
		t.Parallel()

		target := filepath.Join(t.TempDir(), "vol")
		require.NoError(t, os.Mkdir(target, 0o755))

		created, err := mountops.CreateMountPoint(target, mountSpec)
		require.NoError(t, err)
		assert.False(t, created)

		st, err := os.Stat(target)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), st.Mode().Perm())
	})
}

func TestRemoveMountPoint(t *testing.T) {
	t.Parallel()

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		mountops.RemoveMountPoint(zaptest.NewLogger(t), filepath.Join(t.TempDir(), "vol"))
	})

	t.Run("not empty", func(t *testing.T) {
		t.Parallel()

		target := filepath.Join(t.TempDir(), "vol")
		require.NoError(t, os.Mkdir(target, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(target, "data"), []byte("x"), 0o644))

		mountops.RemoveMountPoint(zaptest.NewLogger(t), target)

		assert.FileExists(t, filepath.Join(target, "data"))
	})
}
