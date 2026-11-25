// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package md

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/freddierice/go-losetup/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// attachLoop returns the loop device and its backing file.
func attachLoop(t *testing.T, size int64) (device, image string) {
	t.Helper()

	image = filepath.Join(t.TempDir(), "member.raw")
	require.NoError(t, os.WriteFile(image, nil, 0o644))
	require.NoError(t, os.Truncate(image, size))

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

	return loDev.Path(), image
}

func writeAt(t *testing.T, path string, offset int64, data []byte) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	require.NoError(t, err)

	defer f.Close() //nolint:errcheck

	_, err = f.WriteAt(data, offset)
	require.NoError(t, err)

	require.NoError(t, f.Sync())
}

func readAt(t *testing.T, path string, offset int64, length int) []byte {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)

	defer f.Close() //nolint:errcheck

	data := make([]byte, length)

	_, err = f.ReadAt(data, offset)
	require.NoError(t, err)

	return data
}

// TestRepairRangeKernel runs a range repair on a real RAID1 array with the metadata at the end (as used for
// the boot disk), where the members diverged behind the back of MD.
func TestRepairRangeKernel(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("skipping test; must be root to use loop devices")
	}

	if hostname, _ := os.Hostname(); hostname == "buildkitsandbox" { //nolint:errcheck
		t.Skip("skipping test; md device nodes are not propagated to /dev under buildkit")
	}

	mdadm, err := exec.LookPath("mdadm")
	if err != nil {
		t.Skip("skipping test; mdadm is not available")
	}

	const (
		mib = 1024 * 1024

		rangeOffset = 1 * mib
		rangeLength = 1 * mib
	)

	member0, image0 := attachLoop(t, 64*mib)
	member1, image1 := attachLoop(t, 64*mib)

	m, err := New(WithMdadmPath(mdadm))
	require.NoError(t, err)

	device, err := m.Create(t.Context(), fmt.Sprintf("repair-test-%d", os.Getpid()), CreateOptions{
		Level:       1,
		Metadata:    "1.0",
		RaidDevices: 2,
		Devices:     []string{member0, member1},
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		// t.Context() is canceled before the cleanup runs
		assert.NoError(t, m.Stop(context.Background(), device)) //nolint:usetesting
	})

	// the same data on both members, written through the array
	data := bytes.Repeat([]byte{0xaa}, 8*mib)
	writeAt(t, device, 0, data)

	// with metadata 1.0 the array data starts at sector 0 of the members, so write to the second member
	// behind the back of MD (via the backing file of the loop device, as writes to a block device claimed by
	// MD might be refused): once inside the range, and once outside of it
	diverged := bytes.Repeat([]byte{0x55}, 4096)
	writeAt(t, image1, rangeOffset+4096, diverged)
	writeAt(t, image1, 4*mib, diverged)

	require.NotEqual(t, readAt(t, image0, rangeOffset, rangeLength), readAt(t, image1, rangeOffset, rangeLength))

	mismatches, err := RepairRange(t.Context(), device, rangeOffset, rangeLength)
	require.NoError(t, err)
	assert.NotZero(t, mismatches)

	// the range is identical on both members, with the data from the first one
	assert.Equal(t, data[:rangeLength], readAt(t, image0, rangeOffset, rangeLength))
	assert.Equal(t, data[:rangeLength], readAt(t, image1, rangeOffset, rangeLength))

	// the divergence outside of the range is left alone
	assert.Equal(t, diverged, readAt(t, image1, 4*mib, len(diverged)))

	attrs := mdAttrs(filepath.Join(sysBlockDir, filepath.Base(device), "md"))

	for attr, expected := range map[string]string{
		"sync_action": "idle",
		"sync_min":    "0",
		"sync_max":    "max",
	} {
		actual, err := attrs.read(attr)
		require.NoError(t, err)
		assert.Equal(t, expected, actual, attr)
	}

	speedMin, err := attrs.read("sync_speed_min")
	require.NoError(t, err)
	assert.Contains(t, speedMin, "(system)")

	// nothing to repair anymore
	mismatches, err = RepairRange(t.Context(), device, rangeOffset, rangeLength)
	require.NoError(t, err)
	assert.Zero(t, mismatches)
}
