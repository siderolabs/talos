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
)

func TestProcessSubmounts(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	target := t.TempDir()
	sub := filepath.Join(target, "a", "b")

	require.NoError(t, unix.Mount("target", target, "tmpfs", 0, ""))
	t.Cleanup(func() { unix.Unmount(target, unix.MNT_DETACH) }) //nolint:errcheck

	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, unix.Mount("sub", sub, "tmpfs", 0, ""))

	submounts, err := mount.ProcessSubmounts(int32(os.Getpid()), target) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, []string{sub}, submounts)

	require.NoError(t, unix.Unmount(sub, 0))

	submounts, err = mount.ProcessSubmounts(int32(os.Getpid()), target) //nolint:gosec
	require.NoError(t, err)
	assert.Empty(t, submounts)

	_, err = mount.ProcessSubmounts(-1, target)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestProcessSubmountsBinds(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	// target is a bind of a subdirectory: binds of its siblings do not pin it.
	base := t.TempDir()
	target := t.TempDir()
	inside := t.TempDir()
	outside := t.TempDir()

	require.NoError(t, unix.Mount("base", base, "tmpfs", 0, ""))
	t.Cleanup(func() { unix.Unmount(base, unix.MNT_DETACH) }) //nolint:errcheck

	require.NoError(t, os.MkdirAll(filepath.Join(base, "vol", "x"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(base, "volume"), 0o755))

	for _, bind := range [][2]string{
		{filepath.Join(base, "vol"), target},
		{filepath.Join(base, "vol", "x"), inside},
		{filepath.Join(base, "volume"), outside},
	} {
		require.NoError(t, unix.Mount(bind[0], bind[1], "", unix.MS_BIND, ""))
		t.Cleanup(func() { unix.Unmount(bind[1], unix.MNT_DETACH) }) //nolint:errcheck
	}

	submounts, err := mount.ProcessSubmounts(int32(os.Getpid()), target) //nolint:gosec
	require.NoError(t, err)
	assert.Equal(t, []string{inside}, submounts)
}
