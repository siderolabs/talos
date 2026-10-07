// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

func TestMemoryMaxString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "max", cgroup.UnlimitedMemoryMax().String())
	assert.Equal(t, "4294967296", cgroup.LimitedMemoryMax(4<<30).String())
	assert.NotEqual(t, cgroup.UnlimitedMemoryMax(), cgroup.LimitedMemoryMax(0))
}

func writeMemoryMaxFile(t *testing.T, dir, contents string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, cgroup.MemoryMaxFile), []byte(contents), 0o644))
}

func readMemoryMaxFile(t *testing.T, dir string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(dir, cgroup.MemoryMaxFile))
	require.NoError(t, err)

	return string(contents)
}

func TestReadMemoryMax(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		contents string
		expected cgroup.MemoryMax
		errMsg   string
	}{
		{name: "unlimited", contents: "max\n", expected: cgroup.UnlimitedMemoryMax()},
		{name: "unlimited without newline", contents: "max", expected: cgroup.UnlimitedMemoryMax()},
		{name: "limited", contents: "4294967296\n", expected: cgroup.LimitedMemoryMax(4 << 30)},
		{name: "zero", contents: "0\n", expected: cgroup.LimitedMemoryMax(0)},
		{name: "negative", contents: "-1\n", errMsg: "unexpected value"},
		{name: "garbage", contents: "4G\n", errMsg: "unexpected value"},
		{name: "empty", contents: "", errMsg: "unexpected value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeMemoryMaxFile(t, dir, test.contents)

			actual, err := cgroup.ReadMemoryMax(dir)

			if test.errMsg != "" {
				require.ErrorContains(t, err, test.errMsg)
				assert.ErrorContains(t, err, filepath.Join(dir, cgroup.MemoryMaxFile))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		_, err := cgroup.ReadMemoryMax(t.TempDir())
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestWriteMemoryMax(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeMemoryMaxFile(t, dir, "4294967296\n")

	// shorter value over a longer one: the fake file must not keep a tail of the old value
	require.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.UnlimitedMemoryMax()))
	assert.Equal(t, "max", readMemoryMaxFile(t, dir))

	require.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.LimitedMemoryMax(2<<30)))
	assert.Equal(t, "2147483648", readMemoryMaxFile(t, dir))

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		err := cgroup.WriteMemoryMax(t.TempDir(), cgroup.UnlimitedMemoryMax())
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("opens non-blocking", func(t *testing.T) {
		t.Parallel()

		// a FIFO without a reader is the one file the kernel refuses to open for writing only when
		// O_NONBLOCK is set (ENXIO); a blocking open would hang here, so the write runs with a deadline
		dir := t.TempDir()
		require.NoError(t, unix.Mkfifo(filepath.Join(dir, cgroup.MemoryMaxFile), 0o644))

		errCh := make(chan error, 1)

		go func() {
			errCh <- cgroup.WriteMemoryMax(dir, cgroup.UnlimitedMemoryMax())
		}()

		select {
		case err := <-errCh:
			require.ErrorIs(t, err, unix.ENXIO)
		case <-time.After(10 * time.Second):
			// unblock the writer so the goroutine does not outlive the test
			reader, err := os.OpenFile(filepath.Join(dir, cgroup.MemoryMaxFile), os.O_RDONLY|unix.O_NONBLOCK, 0)
			require.NoError(t, err)
			require.NoError(t, reader.Close())

			t.Fatal("WriteMemoryMax blocked: memory.max is not opened with O_NONBLOCK")
		}
	})
}

func TestEnsureMemoryMax(t *testing.T) {
	t.Parallel()

	t.Run("same value is not written", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeMemoryMaxFile(t, dir, "4294967296\n")
		require.NoError(t, os.Chmod(filepath.Join(dir, cgroup.MemoryMaxFile), 0o444))

		previous, written, err := cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(4<<30))
		require.NoError(t, err)
		assert.False(t, written)
		assert.Equal(t, cgroup.LimitedMemoryMax(4<<30), previous)
		assert.Equal(t, "4294967296\n", readMemoryMaxFile(t, dir))

		previous, written, err = cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(4<<30))
		require.NoError(t, err)
		assert.False(t, written)
		assert.Equal(t, cgroup.LimitedMemoryMax(4<<30), previous)
	})

	t.Run("unlimited stays unlimited", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeMemoryMaxFile(t, dir, "max\n")
		require.NoError(t, os.Chmod(filepath.Join(dir, cgroup.MemoryMaxFile), 0o444))

		previous, written, err := cgroup.EnsureMemoryMax(dir, cgroup.UnlimitedMemoryMax())
		require.NoError(t, err)
		assert.False(t, written)
		assert.Equal(t, cgroup.UnlimitedMemoryMax(), previous)
		assert.Equal(t, "max\n", readMemoryMaxFile(t, dir))
	})

	t.Run("set change reset", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeMemoryMaxFile(t, dir, "max\n")

		previous, written, err := cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(4<<30))
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, cgroup.UnlimitedMemoryMax(), previous)
		assert.Equal(t, "4294967296", readMemoryMaxFile(t, dir))

		previous, written, err = cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(2<<30))
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, cgroup.LimitedMemoryMax(4<<30), previous)
		assert.Equal(t, "2147483648", readMemoryMaxFile(t, dir))

		previous, written, err = cgroup.EnsureMemoryMax(dir, cgroup.UnlimitedMemoryMax())
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, cgroup.LimitedMemoryMax(2<<30), previous)
		assert.Equal(t, "max", readMemoryMaxFile(t, dir))
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		_, written, err := cgroup.EnsureMemoryMax(t.TempDir(), cgroup.UnlimitedMemoryMax())
		require.ErrorIs(t, err, os.ErrNotExist)
		assert.False(t, written)
	})

	t.Run("unparseable current value is not overwritten", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeMemoryMaxFile(t, dir, "garbage\n")

		_, written, err := cgroup.EnsureMemoryMax(dir, cgroup.UnlimitedMemoryMax())
		require.ErrorContains(t, err, "unexpected value")
		assert.False(t, written)
		assert.Equal(t, "garbage\n", readMemoryMaxFile(t, dir))
	})
}

// TestMemoryMaxKernel exercises the helper against a real cgroup v2 directory.
//
// It runs only when TALOS_CGROUP_MEMORY_TEST_DIR names a delegated, otherwise unused cgroup
// whose memory.max the test may change; the limit is restored to "max" afterwards. The results
// are evidence for the kernel the test runs on, not for the kernel Talos ships.
func TestMemoryMaxKernel(t *testing.T) {
	dir := os.Getenv("TALOS_CGROUP_MEMORY_TEST_DIR")
	if dir == "" {
		t.Skip("TALOS_CGROUP_MEMORY_TEST_DIR is not set")
	}

	procs, err := os.ReadFile(filepath.Join(dir, "cgroup.procs"))
	require.NoError(t, err)
	require.Empty(t, procs, "test cgroup must not contain processes")

	pageSize := os.Getpagesize()
	pageCounterMax := uint64(math.MaxInt64) / uint64(pageSize)

	t.Cleanup(func() {
		assert.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.UnlimitedMemoryMax()))
	})

	require.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.UnlimitedMemoryMax()))

	current, err := cgroup.ReadMemoryMax(dir)
	require.NoError(t, err)
	assert.Equal(t, cgroup.UnlimitedMemoryMax(), current)

	// the kernel rounds down to pages: the raw write lands, Ensure reports the mismatch
	require.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.LimitedMemoryMax(uint64(pageSize)+1)))

	current, err = cgroup.ReadMemoryMax(dir)
	require.NoError(t, err)
	assert.Equal(t, cgroup.LimitedMemoryMax(uint64(pageSize)), current)

	_, written, err := cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(uint64(pageSize)*2+1))
	require.ErrorContains(t, err, "reads back")
	assert.True(t, written)

	// a sub-page request becomes a zero limit
	require.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.LimitedMemoryMax(uint64(pageSize)-1)))

	current, err = cgroup.ReadMemoryMax(dir)
	require.NoError(t, err)
	assert.Equal(t, cgroup.LimitedMemoryMax(0), current)

	// int64 max is the PAGE_COUNTER_MAX sentinel and reads back as unlimited
	require.NoError(t, cgroup.WriteMemoryMax(dir, cgroup.LimitedMemoryMax(math.MaxInt64)))

	current, err = cgroup.ReadMemoryMax(dir)
	require.NoError(t, err)
	assert.Equal(t, cgroup.UnlimitedMemoryMax(), current)

	// the largest value NormalizeMemoryLimit accepts is finite on this kernel
	largest, err := kernel.NormalizeMemoryLimit((pageCounterMax-1)*uint64(pageSize), pageSize)
	require.NoError(t, err)

	previous, written, err := cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(largest))
	require.NoError(t, err)
	assert.True(t, written)
	assert.Equal(t, cgroup.UnlimitedMemoryMax(), previous)

	// the value NormalizeMemoryLimit rejects would indeed have been unlimited
	_, err = kernel.NormalizeMemoryLimit(pageCounterMax*uint64(pageSize), pageSize)
	require.ErrorContains(t, err, "treated as unlimited")

	normalized, err := kernel.NormalizeMemoryLimit(1<<30+1, pageSize)
	require.NoError(t, err)

	previous, written, err = cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(normalized))
	require.NoError(t, err)
	assert.True(t, written)
	assert.Equal(t, cgroup.LimitedMemoryMax(largest), previous)

	previous, written, err = cgroup.EnsureMemoryMax(dir, cgroup.LimitedMemoryMax(normalized))
	require.NoError(t, err)
	assert.False(t, written)
	assert.Equal(t, cgroup.LimitedMemoryMax(normalized), previous)

	previous, written, err = cgroup.EnsureMemoryMax(dir, cgroup.UnlimitedMemoryMax())
	require.NoError(t, err)
	assert.True(t, written)
	assert.Equal(t, cgroup.LimitedMemoryMax(normalized), previous)
}
