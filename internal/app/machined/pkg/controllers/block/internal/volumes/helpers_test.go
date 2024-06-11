// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes_test

import (
	"errors"
	randv2 "math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/freddierice/go-losetup/v2"
	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func checkRequirements(t *testing.T) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("test requires root privileges")
	}

	if hostname, _ := os.Hostname(); hostname == "buildkitsandbox" { //nolint: errcheck
		t.Skip("test not supported under buildkit as partition devices are not propagated from /dev")
	}
}

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

func prepareRawImage(t *testing.T, size int64) string {
	t.Helper()

	tmpDir := t.TempDir()

	rawImage := filepath.Join(tmpDir, "image.raw")

	f, err := os.Create(rawImage)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(size))
	require.NoError(t, f.Close())

	loDev := losetupAttachHelper(t, rawImage, false)

	t.Cleanup(func() {
		assert.NoError(t, loDev.Detach())
	})

	return loDev.Path()
}

func formatExt4(t *testing.T, path string) {
	t.Helper()

	cmd := exec.Command("mkfs.ext4", "-L", "extlabel", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	require.NoError(t, cmd.Run())
}

func prepareGPT(t *testing.T, path string, funcs ...func(*gpt.Table)) {
	t.Helper()

	blkdev, err := block.NewFromPath(path, block.OpenForWrite())
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, blkdev.Close())
	})

	gptdev, err := gpt.DeviceFromBlockDevice(blkdev)
	require.NoError(t, err)

	pt, err := gpt.New(gptdev)
	require.NoError(t, err)

	for _, f := range funcs {
		f(pt)
	}

	require.NoError(t, pt.Write())
}
