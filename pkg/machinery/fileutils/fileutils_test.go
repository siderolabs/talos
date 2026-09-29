// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fileutils_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/fileutils"
)

func TestWriteSecret(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "secret.yaml")

	require.NoError(t, fileutils.WriteSecret(path, []byte("foo")))

	assertContentsAndMode(t, path, "foo")

	// overwriting a file which is readable by everyone should restrict the mode
	require.NoError(t, os.Chmod(path, 0o644))
	require.NoError(t, fileutils.WriteSecret(path, []byte("bar")))

	assertContentsAndMode(t, path, "bar")
}

func TestRestrictSecretMode(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "secret.yaml")

	// missing files are ignored
	require.NoError(t, fileutils.RestrictSecretMode(path))

	require.NoError(t, os.WriteFile(path, []byte("foo"), 0o644))
	require.NoError(t, fileutils.RestrictSecretMode(path))

	assertContentsAndMode(t, path, "foo")
}

func assertContentsAndMode(t *testing.T, path, expected string) {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(contents))

	st, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, fileutils.SecretFileMode, st.Mode().Perm())
}
