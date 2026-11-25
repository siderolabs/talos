// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/freddierice/go-losetup/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
)

const fatIoctlGetAttributes = 0x80047210

// mountVFAT formats a loop device with vfat and mounts it, returning the mount point.
//
// The test mounts and unmounts the same device repeatedly, so the mount helper is returned as well.
func mountVFAT(t *testing.T) (dir string, remount func()) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("skipping test; must be root to use loop devices")
	}

	if _, err := exec.LookPath("mkfs.vfat"); err != nil {
		t.Skip("skipping test; mkfs.vfat is not available")
	}

	image := filepath.Join(t.TempDir(), "esp.raw")
	require.NoError(t, os.WriteFile(image, nil, 0o644))
	require.NoError(t, os.Truncate(image, 64*1024*1024))

	var (
		loDev losetup.Device
		err   error
	)

	for range 10 {
		loDev, err = losetup.Attach(image, 0, false)
		if errors.Is(err, unix.EBUSY) {
			time.Sleep(100 * time.Millisecond)

			continue
		}

		break
	}

	require.NoError(t, err)

	t.Cleanup(func() {
		if err := loDev.Detach(); err != nil && !errors.Is(err, unix.ENXIO) {
			assert.NoError(t, err)
		}
	})

	out, err := exec.CommandContext(t.Context(), "mkfs.vfat", loDev.Path()).CombinedOutput()
	require.NoError(t, err, string(out))

	dir = t.TempDir()

	mount := func() {
		require.NoError(t, unix.Mount(loDev.Path(), dir, "vfat", 0, ""))
	}

	mount()

	t.Cleanup(func() {
		assert.NoError(t, unix.Unmount(dir, 0))
	})

	return dir, func() {
		require.NoError(t, unix.Unmount(dir, 0))

		mount()
	}
}

func assertFATReadOnly(t *testing.T, path string) {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)

	defer f.Close() //nolint:errcheck

	attrs, err := unix.IoctlGetUint32(int(f.Fd()), fatIoctlGetAttributes)
	require.NoError(t, err)

	assert.NotZero(t, attrs&0x01, "%s is not read-only", path)
}

func TestPinESPFiles(t *testing.T) {
	dir, remount := mountVFAT(t)

	loaderDir := filepath.Join(dir, "loader")
	require.NoError(t, os.MkdirAll(loaderDir, 0o755))

	seedPath := filepath.Join(loaderDir, "random-seed")
	mixinPath := filepath.Join(loaderDir, "boot-secret-mixin")

	// a full size mixin written by systemd-stub on an earlier boot is kept, a short seed is replaced
	mixin := bytes.Repeat([]byte{0x42}, 32)

	require.NoError(t, os.WriteFile(mixinPath, mixin, 0o644))
	require.NoError(t, os.WriteFile(seedPath, []byte("short"), 0o644))

	require.NoError(t, sdboot.PinESPFiles(dir, t.Logf))

	// make sure the attribute made it to the disk
	remount()

	seed, err := os.ReadFile(seedPath)
	require.NoError(t, err)
	assert.Equal(t, make([]byte, 32), seed)

	actualMixin, err := os.ReadFile(mixinPath)
	require.NoError(t, err)
	assert.Equal(t, mixin, actualMixin)

	for _, path := range []string{seedPath, mixinPath} {
		assertFATReadOnly(t, path)
	}

	// running again on read-only files is a no-op
	require.NoError(t, sdboot.PinESPFiles(dir, t.Logf))

	actualMixin, err = os.ReadFile(mixinPath)
	require.NoError(t, err)
	assert.Equal(t, mixin, actualMixin)

	// both files are created when missing, the mixin is random
	require.NoError(t, os.Remove(seedPath))
	require.NoError(t, os.Remove(mixinPath))

	require.NoError(t, sdboot.PinESPFiles(dir, t.Logf))

	remount()

	actualMixin, err = os.ReadFile(mixinPath)
	require.NoError(t, err)
	assert.Len(t, actualMixin, 32)
	assert.NotEqual(t, make([]byte, 32), actualMixin)

	for _, path := range []string{seedPath, mixinPath} {
		assertFATReadOnly(t, path)
	}
}
