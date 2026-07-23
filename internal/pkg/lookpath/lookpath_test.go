// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lookpath_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/lookpath"
)

func TestInRoot(t *testing.T) {
	root := t.TempDir()

	requireOpenat2InRoot(t, root)

	// Layout under root:
	//   store/tool             real executable
	//   store/data             regular, non-executable file
	//   sbin/other             real executable (in the second PATH dir)
	//   bin/tool   -> /store/tool     absolute symlink (must resolve relative to root)
	//   bin/rel    -> ../store/tool   relative symlink
	//   bin/notexec-> /store/data     absolute symlink to a non-exec file
	//   bin/escape -> /etc/passwd     absolute symlink; resolves to root/etc/passwd (absent)
	for _, dir := range []string{"bin", "sbin", "store"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}

	require.NoError(t, os.WriteFile(filepath.Join(root, "store", "tool"), []byte("x"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "store", "data"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sbin", "other"), []byte("x"), 0o755))
	require.NoError(t, os.Symlink("/store/tool", filepath.Join(root, "bin", "tool")))
	require.NoError(t, os.Symlink("../store/tool", filepath.Join(root, "bin", "rel")))
	require.NoError(t, os.Symlink("/store/data", filepath.Join(root, "bin", "notexec")))
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(root, "bin", "escape")))

	env := []string{"HOME=/root", "PATH=/bin:/sbin"}

	for _, tt := range []struct {
		name    string
		cmd     string
		want    string
		wantErr bool
	}{
		{name: "absolute symlink resolves within root", cmd: "tool", want: "/bin/tool"},
		{name: "relative symlink", cmd: "rel", want: "/bin/rel"},
		{name: "found in second PATH dir", cmd: "other", want: "/sbin/other"},
		{name: "explicit absolute path returned as-is", cmd: "/usr/bin/env", want: "/usr/bin/env"},
		{name: "explicit relative path returned as-is", cmd: "./x", want: "./x"},
		{name: "escape stays confined to root", cmd: "escape", wantErr: true},
		{name: "non-executable not returned", cmd: "notexec", wantErr: true},
		{name: "missing not found", cmd: "nope", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lookpath.InRoot(root, tt.cmd, env)

			if tt.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInRootEmptyPath(t *testing.T) {
	root := t.TempDir()

	requireOpenat2InRoot(t, root)

	_, err := lookpath.InRoot(root, "anything", []string{"HOME=/root"})
	assert.Error(t, err)
}

// requireOpenat2InRoot skips the test if the kernel lacks openat2/RESOLVE_IN_ROOT.
func requireOpenat2InRoot(t *testing.T, root string) {
	t.Helper()

	fd, err := unix.Open(root, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	require.NoError(t, err)

	defer unix.Close(fd) //nolint:errcheck

	if _, err := unix.Openat2(fd, ".", &unix.OpenHow{
		Flags:   unix.O_PATH | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_IN_ROOT,
	}); errors.Is(err, unix.ENOSYS) {
		t.Skip("openat2(RESOLVE_IN_ROOT) not supported by this kernel")
	} else {
		require.NoError(t, err)
	}
}
