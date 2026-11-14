// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package mount_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	mount "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/xfs"
)

func TestDetachedMountHardened(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	mgr := mount.NewManager(mount.WithDetached(), mount.WithFsopen("tmpfs"))

	point, err := mgr.Mount()
	require.NoError(t, err)

	t.Cleanup(func() { mgr.Unmount() }) //nolint:errcheck

	fd, err := point.Root().Fd()
	require.NoError(t, err)

	var st unix.Statfs_t

	require.NoError(t, unix.Fstatfs(fd, &st))

	for name, flag := range map[string]int64{
		"nosuid": unix.ST_NOSUID,
		"nodev":  unix.ST_NODEV,
		"noexec": unix.ST_NOEXEC,
	} {
		assert.NotZero(t, st.Flags&flag, "detached mount is missing %s", name)
	}

	assert.Zero(t, st.Flags&unix.ST_RDONLY, "detached mount must stay writable")

	// the device node can be created, but not opened through the mount
	require.NoError(t, unix.Mknodat(fd, "null", unix.S_IFCHR|0o600, int(unix.Mkdev(1, 3))))

	_, err = xfs.OpenFile(point.Root(), "null", os.O_RDONLY, 0)
	require.ErrorIs(t, err, unix.EACCES)
}

// TestMountAttributesPropagated mounts under a shared parent with a slave copy: the copy the kernel clones on attach
// must carry the mount attributes, as a later setattr only changes the original.
func TestMountAttributesPropagated(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	parent := t.TempDir()
	slave := t.TempDir()

	require.NoError(t, unix.Mount("parent", parent, "tmpfs", 0, ""))
	t.Cleanup(func() { unix.Unmount(parent, unix.MNT_DETACH) }) //nolint:errcheck
	require.NoError(t, unix.Mount("", parent, "", unix.MS_SHARED, ""))

	require.NoError(t, unix.Mount(parent, slave, "", unix.MS_BIND, ""))
	t.Cleanup(func() { unix.Unmount(slave, unix.MNT_DETACH) }) //nolint:errcheck
	require.NoError(t, unix.Mount("", slave, "", unix.MS_SLAVE, ""))

	require.NoError(t, os.Mkdir(filepath.Join(parent, "child"), 0o755))

	mgr := mount.NewManager(mount.WithTarget(filepath.Join(parent, "child")), mount.WithFsopen("tmpfs"), mount.WithSecure())

	point, err := mgr.Mount()
	require.NoError(t, err)

	t.Cleanup(func() { mgr.Unmount() }) //nolint:errcheck

	for _, path := range []string{filepath.Join(parent, "child"), filepath.Join(slave, "child")} {
		var st unix.Statfs_t

		require.NoError(t, unix.Statfs(path, &st))

		assert.NotZero(t, st.Flags&unix.ST_NOSUID, "%s is missing nosuid", path)
		assert.NotZero(t, st.Flags&unix.ST_NODEV, "%s is missing nodev", path)
	}

	// later changes reach only the original: replaying the recorded ones brings the copy in line
	require.NoError(t, point.SetNoExec(true))
	require.NoError(t, point.RemountReadOnly())

	slaveChild := filepath.Join(slave, "child")

	var st unix.Statfs_t

	require.NoError(t, unix.Statfs(slaveChild, &st))
	assert.Zero(t, st.Flags&(unix.ST_NOEXEC|unix.ST_RDONLY), "setattr reached the copy")

	changes := point.AttrChanges()
	require.NoError(t, unix.MountSetattr(unix.AT_FDCWD, slaveChild, 0, &unix.MountAttr{Attr_set: changes.Set, Attr_clr: changes.Clr}))

	require.NoError(t, unix.Statfs(slaveChild, &st))
	assert.NotZero(t, st.Flags&unix.ST_NOEXEC, "copy is missing noexec")
	assert.NotZero(t, st.Flags&unix.ST_RDONLY, "copy is missing rdonly")

	point.ResetAttrChanges()
	assert.True(t, point.AttrChanges().Empty())
}
