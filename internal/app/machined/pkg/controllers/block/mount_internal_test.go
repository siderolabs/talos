// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRemoveUserVolumeMountPoint(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "vol")
		require.NoError(t, os.Mkdir(target, 0o755))

		removeUserVolumeMountPoint(logger, root, target)

		assert.NoDirExists(t, target)
	})

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()

		removeUserVolumeMountPoint(logger, root, filepath.Join(root, "vol"))
	})

	t.Run("not empty", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "vol")
		require.NoError(t, os.Mkdir(target, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(target, "data"), []byte("x"), 0o644))

		removeUserVolumeMountPoint(logger, root, target)

		assert.FileExists(t, filepath.Join(target, "data"))
	})

	t.Run("outside user volume root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "nested", "vol")
		require.NoError(t, os.MkdirAll(target, 0o755))

		removeUserVolumeMountPoint(logger, root, target)

		assert.DirExists(t, target)
	})
}
