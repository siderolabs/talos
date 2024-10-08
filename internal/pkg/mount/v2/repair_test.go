// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount_test

import (
	"errors"
	randv2 "math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/freddierice/go-losetup/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/pkg/makefs"
)

// Some tests in this package cannot be run under buildkit, as buildkit doesn't propagate partition devices
// like /dev/loopXpY into the sandbox. To run the tests on your local computer, do the following:
//
//  go test -exec sudo -v --count 1 github.com/siderolabs/talos/internal/pkg/mount/v2

const diskSize = 4 * 1024 * 1024 * 1024 // 4 GiB

func losetupAttachHelper(t *testing.T, rawImage string, readonly bool) losetup.Device {
	t.Helper()

	for range 10 {
		loDev, err := losetup.Attach(rawImage, 0, readonly)
		if err != nil {
			if errors.Is(err, unix.EBUSY) {
				spraySleep := max(randv2.ExpFloat64(), 2.0)

				t.Logf("retrying after %v seconds", spraySleep)

				time.Sleep(time.Duration(spraySleep * float64(time.Second)))

				continue
			}
		}

		require.NoError(t, err)

		return loDev
	}

	t.Fatal("failed to attach loop device") //nolint:revive

	panic("unreachable")
}

func TestRepair(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("can't run the test as non-root")
	}

	hostname, _ := os.Hostname() //nolint:errcheck

	if hostname == "buildkitsandbox" {
		t.Skip("test not supported under buildkit as partition devices are not propagated from /dev")
	}

	tmpDir := t.TempDir()

	rawImage := filepath.Join(tmpDir, "image.raw")

	f, err := os.Create(rawImage)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(int64(diskSize)))
	require.NoError(t, f.Close())

	loDev := losetupAttachHelper(t, rawImage, false)

	t.Cleanup(func() {
		assert.NoError(t, loDev.Detach())
	})

	mountDir := filepath.Join(tmpDir, "var")

	require.NoError(t, os.MkdirAll(mountDir, 0o700))
	require.NoError(t, makefs.XFS(loDev.Path()))

	mountPoint := mount.NewPoint(loDev.Path(), mountDir, "xfs", mount.WithFlags(unix.MS_NOATIME))

	unmounter1, err := mountPoint.Mount(mount.WithMountPrinter(t.Logf))
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, unmounter1())
	})

	assert.NoError(t, unmounter1())

	// // now corrupt the disk
	cmd := exec.Command("xfs_db", []string{
		"-x",
		"-c blockget",
		"-c blocktrash -s 512109 -n 100",
		loDev.Path(),
	}...)

	assert.NoError(t, cmd.Run())

	unmounter2, err := mountPoint.Mount(mount.WithMountPrinter(t.Logf))
	require.NoError(t, err)

	require.NoError(t, unmounter2())
}
