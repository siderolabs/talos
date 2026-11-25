// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package md_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/md"
)

// repairSpeedMin is the sync_speed_min set for the duration of a range repair.
const repairSpeedMin = "100000"

// fakeArray lays out the sysfs attributes of an idle RAID1 array md0 of 1GiB, and returns the MD instance
// which reads them, and the device path.
func fakeArray(t *testing.T, attrs map[string]string) (m *md.MD, device, mdDir string) {
	t.Helper()

	sysBlockDir := t.TempDir()

	m, err := md.New(md.WithSysBlockDir(sysBlockDir))
	require.NoError(t, err)

	mdDir = filepath.Join(sysBlockDir, "md0", "md")
	require.NoError(t, os.MkdirAll(mdDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sysBlockDir, "md0", "size"), []byte("2097152\n"), 0o644))

	for attr, value := range map[string]string{
		"level":          "raid1",
		"sync_action":    "idle",
		"sync_completed": "none",
		"sync_min":       "0",
		"sync_max":       "max",
		"sync_speed_min": "1000 (system)",
		"mismatch_cnt":   "0",
	} {
		if override, ok := attrs[attr]; ok {
			value = override
		}

		writeAttr(t, mdDir, attr, value)
	}

	devDir := t.TempDir()
	device = filepath.Join(devDir, "md0")
	require.NoError(t, os.WriteFile(device, nil, 0o644))

	// the device is looked up in sysfs by the name it resolves to
	byID := filepath.Join(devDir, "md-name-talos:boot")
	require.NoError(t, os.Symlink(device, byID))

	return m, byID, mdDir
}

// writeAttr replaces the attribute atomically, so that the concurrent reader never sees it empty.
func writeAttr(t *testing.T, mdDir, attr, value string) {
	t.Helper()

	tmp := filepath.Join(mdDir, "."+attr)

	assert.NoError(t, os.WriteFile(tmp, []byte(value+"\n"), 0o644))
	assert.NoError(t, os.Rename(tmp, filepath.Join(mdDir, attr)))
}

func readAttr(t assert.TestingT, mdDir, attr string) string {
	out, err := os.ReadFile(filepath.Join(mdDir, attr))
	assert.NoError(t, err)

	return strings.TrimSpace(string(out))
}

// assertRestored checks that the sync window and the speed floor are back to their defaults, and the array is idle.
func assertRestored(t *testing.T, mdDir string) {
	t.Helper()

	assert.Equal(t, "idle", readAttr(t, mdDir, "sync_action"))
	assert.Equal(t, "0", readAttr(t, mdDir, "sync_min"))
	assert.Equal(t, "max", readAttr(t, mdDir, "sync_max"))
	assert.Equal(t, "system", readAttr(t, mdDir, "sync_speed_min"))
}

// fakeKernel waits for the repair to be requested, records the sync window, and then calls progress.
func fakeKernel(t *testing.T, mdDir string, progress func()) <-chan map[string]string {
	t.Helper()

	started := make(chan map[string]string, 1)

	go func() {
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.Equal(c, "repair", readAttr(c, mdDir, "sync_action"))
		}, 5*time.Second, time.Millisecond)

		started <- map[string]string{
			"sync_min":       readAttr(t, mdDir, "sync_min"),
			"sync_max":       readAttr(t, mdDir, "sync_max"),
			"sync_speed_min": readAttr(t, mdDir, "sync_speed_min"),
		}

		progress()
	}()

	return started
}

func TestRepairRange(t *testing.T) {
	t.Parallel()

	m, device, mdDir := fakeArray(t, nil)

	// 1MiB offset is sector 2048; the end (1MiB + 100MiB + 1 byte) rounds up to the next 4KiB, sector 206856
	const (
		offset = 1 << 20
		length = 100<<20 + 1
	)

	started := fakeKernel(t, mdDir, func() {
		writeAttr(t, mdDir, "sync_completed", "1000 / 2097152")
		writeAttr(t, mdDir, "mismatch_cnt", "16")
		// the kernel pauses at sync_max: progress reaches it, but the action stays "repair"
		writeAttr(t, mdDir, "sync_completed", "206856 / 2097152")
	})

	mismatches, err := m.RepairRange(t.Context(), device, offset, length)
	require.NoError(t, err)
	assert.EqualValues(t, 16, mismatches)

	assert.Equal(t, map[string]string{
		"sync_min":       "2048",
		"sync_max":       "206856",
		"sync_speed_min": repairSpeedMin,
	}, <-started)

	assertRestored(t, mdDir)
}

func TestRepairRangeRestoresLocalSpeedMin(t *testing.T) {
	t.Parallel()

	m, device, mdDir := fakeArray(t, map[string]string{"sync_speed_min": "5000 (local)"})

	fakeKernel(t, mdDir, func() {
		writeAttr(t, mdDir, "sync_completed", "4096 / 2097152")
	})

	_, err := m.RepairRange(t.Context(), device, 0, 4096*512)
	require.NoError(t, err)

	assert.Equal(t, "5000", readAttr(t, mdDir, "sync_speed_min"))
}

func TestRepairRangeStopped(t *testing.T) {
	t.Parallel()

	m, device, mdDir := fakeArray(t, nil)

	// something else stops the repair before it reaches the end of the range
	fakeKernel(t, mdDir, func() {
		writeAttr(t, mdDir, "sync_action", "idle")
	})

	_, err := m.RepairRange(t.Context(), device, 0, 4096*512)
	require.ErrorContains(t, err, "stopped before reaching sector 4096")

	assertRestored(t, mdDir)
}

func TestRepairRangeReplaced(t *testing.T) {
	t.Parallel()

	m, device, mdDir := fakeArray(t, nil)

	// a member was added just before the repair was requested, so the kernel runs a recovery instead,
	// which pauses at sync_max as well
	fakeKernel(t, mdDir, func() {
		writeAttr(t, mdDir, "sync_action", "recover")
		writeAttr(t, mdDir, "sync_completed", "4096 / 2097152")
	})

	_, err := m.RepairRange(t.Context(), device, 0, 4096*512)
	require.ErrorIs(t, err, md.ErrResync)
	require.ErrorContains(t, err, `replaced with "recover"`)

	assertRestored(t, mdDir)
}

func TestRepairRangeStale(t *testing.T) {
	t.Parallel()

	// a range repair was left paused at sync_max, with the speed floor raised
	m, device, mdDir := fakeArray(t, map[string]string{
		"sync_action":    "repair",
		"sync_completed": "1024 / 2097152",
		"sync_min":       "512",
		"sync_max":       "1024",
		"sync_speed_min": repairSpeedMin + " (local)",
	})

	go func() {
		// the new repair is requested once the stale one is stopped
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.Equal(c, "4096", readAttr(c, mdDir, "sync_max"))
			assert.Equal(c, "repair", readAttr(c, mdDir, "sync_action"))
		}, 5*time.Second, time.Millisecond)

		writeAttr(t, mdDir, "mismatch_cnt", "8")
		writeAttr(t, mdDir, "sync_completed", "4096 / 2097152")
	}()

	mismatches, err := m.RepairRange(t.Context(), device, 0, 4096*512)
	require.NoError(t, err)
	assert.EqualValues(t, 8, mismatches)

	// the window is reset to the defaults, and not restored to what the stale repair left
	assertRestored(t, mdDir)
}

func TestRepairRangeTimeout(t *testing.T) {
	t.Parallel()

	m, device, mdDir := fakeArray(t, nil)

	// the repair never makes progress
	fakeKernel(t, mdDir, func() {})

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err := m.RepairRange(ctx, device, 0, 4096*512)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	assertRestored(t, mdDir)
}

func TestRepairRangeRefused(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		attrs  map[string]string
		offset uint64
		length uint64

		expected error
	}{
		{
			name:     "busy",
			attrs:    map[string]string{"sync_action": "resync"},
			length:   4096,
			expected: md.ErrResync,
		},
		{
			name:     "repair of the whole array",
			attrs:    map[string]string{"sync_action": "repair", "sync_completed": "4096 / 2097152"},
			length:   4096,
			expected: md.ErrResync,
		},
		{
			name:     "not raid1",
			attrs:    map[string]string{"level": "raid0"},
			length:   4096,
			expected: md.ErrInvalidArgument,
		},
		{
			name:     "empty range",
			expected: md.ErrInvalidArgument,
		},
		{
			name:     "range reaches the end of the array",
			offset:   1<<30 - 4096,
			length:   4096,
			expected: md.ErrInvalidArgument,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			m, device, mdDir := fakeArray(t, test.attrs)

			_, err := m.RepairRange(t.Context(), device, test.offset, test.length)
			require.ErrorIs(t, err, test.expected)

			// nothing was touched
			assert.Equal(t, "0", readAttr(t, mdDir, "sync_min"))
			assert.Equal(t, "max", readAttr(t, mdDir, "sync_max"))
			assert.Equal(t, "1000 (system)", readAttr(t, mdDir, "sync_speed_min"))
		})
	}
}

func TestIsArray(t *testing.T) {
	t.Parallel()

	m, device, mdDir := fakeArray(t, nil)

	isArray, err := m.IsArray(device)
	require.NoError(t, err)
	assert.True(t, isArray)

	disk := filepath.Join(t.TempDir(), "sda")
	require.NoError(t, os.WriteFile(disk, nil, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(mdDir, "..", "..", "sda"), 0o755))

	isArray, err = m.IsArray(disk)
	require.NoError(t, err)
	assert.False(t, isArray)

	_, err = m.IsArray(filepath.Join(t.TempDir(), "missing"))
	assert.Error(t, err)
}
