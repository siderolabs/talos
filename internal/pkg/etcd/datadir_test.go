// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/etcd"
)

func TestDataDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dataDir := etcd.NewDataDir(dir)

	// empty directory
	hasData, err := dataDir.HasMemberData()
	require.NoError(t, err)
	assert.False(t, hasData)

	_, hasMarker, err := dataDir.LearnerMarker()
	require.NoError(t, err)
	assert.False(t, hasMarker)

	require.NoError(t, dataDir.RemoveLearnerMarker())
	require.NoError(t, dataDir.Wipe())

	// learner marker
	require.NoError(t, dataDir.WriteLearnerMarker(0xdeadbeef01020304))

	memberID, hasMarker, err := dataDir.LearnerMarker()
	require.NoError(t, err)
	assert.True(t, hasMarker)
	assert.Equal(t, uint64(0xdeadbeef01020304), memberID)

	// the marker is not the etcd data
	hasData, err = dataDir.HasMemberData()
	require.NoError(t, err)
	assert.False(t, hasData)

	// etcd data
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "member", "wal"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "member", "wal", "0.wal"), []byte("wal"), 0o600))

	hasData, err = dataDir.HasMemberData()
	require.NoError(t, err)
	assert.True(t, hasData)

	require.NoError(t, dataDir.RemoveLearnerMarker())

	_, hasMarker, err = dataDir.LearnerMarker()
	require.NoError(t, err)
	assert.False(t, hasMarker)

	hasData, err = dataDir.HasMemberData()
	require.NoError(t, err)
	assert.True(t, hasData)

	// wipe removes everything, but the directory itself
	require.NoError(t, dataDir.WriteLearnerMarker(1))
	require.NoError(t, dataDir.Wipe())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDataDirCorruptedMarker(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dataDir := etcd.NewDataDir(dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, etcd.LearnerMarkerFilename), []byte("garbage"), 0o600))

	// the marker is still there, but the member ID is not known
	memberID, hasMarker, err := dataDir.LearnerMarker()
	require.NoError(t, err)
	assert.True(t, hasMarker)
	assert.Zero(t, memberID)
}

func TestDataDirMissing(t *testing.T) {
	t.Parallel()

	dataDir := etcd.NewDataDir(filepath.Join(t.TempDir(), "missing"))

	hasData, err := dataDir.HasMemberData()
	require.NoError(t, err)
	assert.False(t, hasData)

	require.NoError(t, dataDir.Wipe())
}
