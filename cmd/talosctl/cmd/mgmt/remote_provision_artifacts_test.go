// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package mgmt //nolint:testpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncBootArtifact(t *testing.T) {
	t.Parallel()

	server := &remoteProvisionImpl{stateDir: t.TempDir()}

	first := writeCachedArtifact(t, server, []byte("first"))
	second := writeCachedArtifact(t, server, []byte("second"))

	stablePath, changed, err := server.syncBootArtifact("test-cluster", "kernel", first)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []byte("first"), mustReadFile(t, stablePath))
	require.True(t, mustLstat(t, stablePath).Mode().IsRegular())

	stablePath, changed, err = server.syncBootArtifact("test-cluster", "kernel", first)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, []byte("first"), mustReadFile(t, stablePath))

	stablePath, changed, err = server.syncBootArtifact("test-cluster", "kernel", second)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []byte("second"), mustReadFile(t, stablePath))
}

func TestSyncBootArtifactRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	server := &remoteProvisionImpl{stateDir: t.TempDir()}
	artifact := writeCachedArtifact(t, server, []byte("kernel"))

	_, _, err := server.syncBootArtifact("../escape", "kernel", artifact)
	require.Error(t, err)

	_, _, err = server.syncBootArtifact("..", "kernel", artifact)
	require.Error(t, err)
	_, err = os.Stat(filepath.Join(server.stateDir, "kernel"))
	require.ErrorIs(t, err, os.ErrNotExist)

	_, _, err = server.syncBootArtifact("test-cluster", "iso", artifact)
	require.ErrorContains(t, err, "unsupported boot artifact")
}

func writeCachedArtifact(t *testing.T, server *remoteProvisionImpl, data []byte) string {
	t.Helper()

	digest := sha256.Sum256(data)
	path := filepath.Join(server.artifactCacheDir(), hex.EncodeToString(digest[:]))

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))

	return path
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return data
}

func mustLstat(t *testing.T, path string) os.FileInfo {
	t.Helper()

	info, err := os.Lstat(path)
	require.NoError(t, err)

	return info
}
