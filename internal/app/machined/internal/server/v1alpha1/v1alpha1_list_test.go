// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime //nolint:testpackage // to test the unexported device filter

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/archiver"
)

func TestIsHidden(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(path, []byte("secret"), 0o600))

	info, err := os.Lstat(path)
	require.NoError(t, err)

	device := info.Sys().(*syscall.Stat_t).Dev //nolint:forcetypeassert,errcheck
	item := archiver.FileItem{FullPath: path, FileInfo: info}

	assert.False(t, isHidden(item, nil), "admins see everything")
	assert.False(t, isHidden(item, map[uint64]struct{}{device + 1: {}}), "other devices are visible")
	assert.True(t, isHidden(item, map[uint64]struct{}{device: {}}), "hidden device")
	assert.False(t, isHidden(archiver.FileItem{FullPath: path, Error: errors.New("boom")}, map[uint64]struct{}{device: {}}),
		"errors carry no file info")
}
