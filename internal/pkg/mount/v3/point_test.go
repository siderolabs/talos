// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package mount_test

import (
	"os"
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
