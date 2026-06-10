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
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

// TestNewSecureWritableOverlay composes a writable overlay over a cloned lower dir (the helper
// creates and then releases its own anonymous upper tmpfs), closes the lower fd, and confirms the
// overlay still works: the lower file is visible, writes go to the upper and persist, and a lower
// file copies up. This proves the helper's internal base tmpfs (and the caller's lower fd) can be
// released once the overlay is composed — overlayfs holds its own references.
func TestNewSecureWritableOverlay(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root for mount operations")
	}

	lower := filepath.Join(t.TempDir(), "lower")
	require.NoError(t, os.MkdirAll(lower, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(lower, "static.txt"), []byte("STATIC"), 0o644))

	lowerFd, err := unix.OpenTree(unix.AT_FDCWD, lower, unix.OPEN_TREE_CLONE|unix.OPEN_TREE_CLOEXEC)
	require.NoError(t, err)

	ov, err := mount.NewSecureWritableOverlay([]int{lowerFd}, []fsopen.Option{
		fsopen.WithStringParameter("mode", "0755"),
		fsopen.WithStringParameter("size", "8M"),
	}, t.Logf)
	require.NoError(t, err)
	t.Cleanup(func() { ov.Close() }) //nolint:errcheck

	// the lower fd may be released — the composed overlay keeps it alive.
	require.NoError(t, unix.Close(lowerFd))

	// lower file is visible through the overlay.
	b, err := xfs.ReadFile(ov, "static.txt")
	require.NoError(t, err)
	assert.Equal(t, "STATIC", string(b))

	// a new file written through the overlay (into the upper) persists despite the helper having
	// released its internal base tmpfs.
	require.NoError(t, xfs.WriteFile(ov, "written.txt", []byte("VIAUPPER"), 0o644))
	w, err := xfs.ReadFile(ov, "written.txt")
	require.NoError(t, err)
	assert.Equal(t, "VIAUPPER", string(w))

	// overriding a lower file copies it up to the upper.
	require.NoError(t, xfs.WriteFile(ov, "static.txt", []byte("OVERRIDE"), 0o644))
	o, err := xfs.ReadFile(ov, "static.txt")
	require.NoError(t, err)
	assert.Equal(t, "OVERRIDE", string(o))

	t.Log("RESULT: writable overlay works after the helper's base tmpfs + the lower fd are closed")
}
