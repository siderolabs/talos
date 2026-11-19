// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

func TestMountBinder(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root")
	}

	point, err := mountv3.NewManager(
		mountv3.WithDetached(),
		mountv3.WithFsopen("tmpfs", fsopen.WithStringParameter("size", "1M")),
	).Mount()
	require.NoError(t, err)

	t.Cleanup(func() { point.Root().Close() }) //nolint:errcheck

	fd, err := point.Root().Fd()
	require.NoError(t, err)

	source := fmt.Sprintf("/proc/self/fd/%d", fd)

	root, err := os.OpenRoot(source)
	require.NoError(t, err)

	t.Cleanup(func() { root.Close() }) //nolint:errcheck

	require.NoError(t, root.WriteFile("a.conf", []byte("v1"), 0o644))

	var stat unix.Stat_t

	require.NoError(t, unix.Stat(source, &stat))

	target := filepath.Join(t.TempDir(), "binds", "ctr.app")

	var binder filesctrl.MountBinder

	bound, err := binder.Bound(target, stat.Dev)
	require.NoError(t, err)
	assert.False(t, bound)

	require.NoError(t, binder.Bind(source, target))

	t.Cleanup(func() { binder.Unbind(target) }) //nolint:errcheck

	bound, err = binder.Bound(target, stat.Dev)
	require.NoError(t, err)
	assert.True(t, bound)

	assertFile(t, target, "a.conf", "v1", 0o644)
	require.ErrorIs(t, os.WriteFile(filepath.Join(target, "b.conf"), nil, 0o644), unix.EROFS)

	require.NoError(t, root.WriteFile("a.conf", []byte("v2"), 0o644))
	assertFile(t, target, "a.conf", "v2", 0o644)

	var statfs unix.Statfs_t

	require.NoError(t, unix.Statfs(target, &statfs))
	assert.NotZero(t, statfs.Flags&unix.ST_RDONLY)
	assert.NotZero(t, statfs.Flags&unix.ST_NOSUID)
	assert.NotZero(t, statfs.Flags&unix.ST_NODEV)
	assert.NotZero(t, statfs.Flags&unix.ST_NOEXEC)

	require.NoError(t, binder.Bind(source, target))

	require.NoError(t, binder.Unbind(target))
	assert.NoDirExists(t, target)

	bound, err = binder.Bound(target, stat.Dev)
	require.NoError(t, err)
	assert.False(t, bound)

	require.NoError(t, binder.Unbind(target))
}
