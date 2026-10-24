// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
)

func TestWriteKubeletCredentialProviderFiles(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "credentialproviders")

	require.NoError(t, k8sctrl.WriteKubeletCredentialProviderFiles(dir, map[string]string{
		"azure.json": "foo",
		"other.json": "bar",
	}))

	assertFiles(t, dir, map[string]string{
		"azure.json": "foo",
		"other.json": "bar",
	})

	require.NoError(t, k8sctrl.WriteKubeletCredentialProviderFiles(dir, map[string]string{
		"azure.json": "baz",
	}))

	assertFiles(t, dir, map[string]string{
		"azure.json": "baz",
	})

	require.NoError(t, k8sctrl.WriteKubeletCredentialProviderFiles(dir, nil))

	assert.NoDirExists(t, dir)
}

func assertFiles(t *testing.T, dir string, expected map[string]string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, len(expected))

	for name, content := range expected {
		path := filepath.Join(dir, name)

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, string(data))

		st, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	}
}
