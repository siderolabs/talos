// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package tpm_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/tpm"
)

func TestOpenDeviceMissing(t *testing.T) {
	t.Parallel()

	_, err := tpm.OpenDevice(filepath.Join(t.TempDir(), "tpmrm0"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestOpenDeviceNotDevice(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tpmrm0")

	require.NoError(t, os.WriteFile(path, nil, 0o600))

	_, err := tpm.OpenDevice(path)
	require.ErrorIs(t, err, tpm.ErrNotDevice)
	assert.NotErrorIs(t, err, os.ErrNotExist)
}

func TestOpenDeviceBlocking(t *testing.T) {
	t.Parallel()

	// like the TPM, the pty master supports poll(2), so os.OpenFile would put it into non-blocking mode
	const path = "/dev/ptmx"

	before := openFDs(t, path)

	dev, err := tpm.OpenDevice(path)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, dev.Close()) })

	var opened []int

	for fd := range openFDs(t, path) {
		if _, ok := before[fd]; !ok {
			opened = append(opened, fd)
		}
	}

	require.Len(t, opened, 1)

	// the kernel executes TPM commands asynchronously for non-blocking descriptors,
	// which requires waiting for the response with poll(2), which fails with EINTR
	flags, err := unix.FcntlInt(uintptr(opened[0]), unix.F_GETFL, 0)
	require.NoError(t, err)

	assert.Zero(t, flags&unix.O_NONBLOCK, "TPM device should be opened in blocking mode")
}

func TestOpenDeviceSend(t *testing.T) {
	t.Parallel()

	dev, err := tpm.OpenDevice("/dev/zero")
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, dev.Close()) })

	// the response is all zeroes, but the command goes through the transport
	resp, err := dev.Send([]byte{0x80, 0x01, 0x00, 0x00, 0x00, 0x0a, 0x00, 0x00, 0x01, 0x7b})
	require.NoError(t, err)
	assert.NotEmpty(t, resp)
}

// openFDs returns the open descriptors of the character device at path.
//
// The descriptors are matched by the device number, as the path in /proc/self/fd
// might differ from the path opened (e.g. /dev/ptmx is a symlink to /dev/pts/ptmx in containers).
func openFDs(t *testing.T, path string) map[int]struct{} {
	t.Helper()

	var st unix.Stat_t

	require.NoError(t, unix.Stat(path, &st))
	require.Equal(t, uint32(unix.S_IFCHR), st.Mode&unix.S_IFMT)

	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)

	fds := map[int]struct{}{}

	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		require.NoError(t, err)

		var fdSt unix.Stat_t

		// the descriptor of the directory listing is already closed
		if unix.Fstat(fd, &fdSt) != nil {
			continue
		}

		if fdSt.Mode&unix.S_IFMT == unix.S_IFCHR && fdSt.Rdev == st.Rdev {
			fds[fd] = struct{}{}
		}
	}

	return fds
}
