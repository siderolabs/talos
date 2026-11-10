// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
)

func TestPidsMaxString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "max", cgroup.UnlimitedPidsMax().String())
	assert.Equal(t, "262044", cgroup.LimitedPidsMax(262044).String())
	assert.NotEqual(t, cgroup.UnlimitedPidsMax(), cgroup.LimitedPidsMax(0))
}

func writePidsMaxFile(t *testing.T, dir, contents string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, cgroup.PidsMaxFile), []byte(contents), 0o644))
}

func readPidsMaxFile(t *testing.T, dir string) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join(dir, cgroup.PidsMaxFile))
	require.NoError(t, err)

	return string(contents)
}

func TestReadPidsMax(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		contents string
		expected cgroup.PidsMax
		errMsg   string
	}{
		{name: "unlimited", contents: "max\n", expected: cgroup.UnlimitedPidsMax()},
		{name: "unlimited without newline", contents: "max", expected: cgroup.UnlimitedPidsMax()},
		{name: "limited", contents: "12345\n", expected: cgroup.LimitedPidsMax(12345)},
		{name: "zero", contents: "0\n", expected: cgroup.LimitedPidsMax(0)},
		{name: "negative", contents: "-1\n", errMsg: "unexpected value"},
		{name: "garbage", contents: "lots\n", errMsg: "unexpected value"},
		{name: "empty", contents: "", errMsg: "unexpected value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writePidsMaxFile(t, dir, test.contents)

			actual, err := cgroup.ReadPidsMax(dir)

			if test.errMsg != "" {
				require.ErrorContains(t, err, test.errMsg)
				assert.ErrorContains(t, err, filepath.Join(dir, cgroup.PidsMaxFile))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		_, err := cgroup.ReadPidsMax(t.TempDir())
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestWritePidsMax(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writePidsMaxFile(t, dir, "262044\n")

	// shorter value over a longer one: the fake file must not keep a tail of the old value
	require.NoError(t, cgroup.WritePidsMax(dir, cgroup.UnlimitedPidsMax()))
	assert.Equal(t, "max", readPidsMaxFile(t, dir))

	require.NoError(t, cgroup.WritePidsMax(dir, cgroup.LimitedPidsMax(14900)))
	assert.Equal(t, "14900", readPidsMaxFile(t, dir))

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		err := cgroup.WritePidsMax(t.TempDir(), cgroup.UnlimitedPidsMax())
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestEnsurePidsMax(t *testing.T) {
	t.Parallel()

	t.Run("same value is not written", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writePidsMaxFile(t, dir, "262044\n")
		// read-only: any write attempt would fail the call
		require.NoError(t, os.Chmod(filepath.Join(dir, cgroup.PidsMaxFile), 0o444))

		previous, written, err := cgroup.EnsurePidsMax(dir, cgroup.LimitedPidsMax(262044))
		require.NoError(t, err)
		assert.False(t, written)
		assert.Equal(t, cgroup.LimitedPidsMax(262044), previous)
		assert.Equal(t, "262044\n", readPidsMaxFile(t, dir))
	})

	t.Run("set change reset", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writePidsMaxFile(t, dir, "max\n")

		previous, written, err := cgroup.EnsurePidsMax(dir, cgroup.LimitedPidsMax(262044))
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, cgroup.UnlimitedPidsMax(), previous)
		assert.Equal(t, "262044", readPidsMaxFile(t, dir))

		previous, written, err = cgroup.EnsurePidsMax(dir, cgroup.LimitedPidsMax(14900))
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, cgroup.LimitedPidsMax(262044), previous)
		assert.Equal(t, "14900", readPidsMaxFile(t, dir))

		previous, written, err = cgroup.EnsurePidsMax(dir, cgroup.UnlimitedPidsMax())
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, cgroup.LimitedPidsMax(14900), previous)
		assert.Equal(t, "max", readPidsMaxFile(t, dir))
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		_, written, err := cgroup.EnsurePidsMax(t.TempDir(), cgroup.UnlimitedPidsMax())
		require.ErrorIs(t, err, os.ErrNotExist)
		assert.False(t, written)
	})

	t.Run("unparseable current value is not overwritten", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writePidsMaxFile(t, dir, "garbage\n")

		_, written, err := cgroup.EnsurePidsMax(dir, cgroup.LimitedPidsMax(1))
		require.ErrorContains(t, err, "unexpected value")
		assert.False(t, written)
		assert.Equal(t, "garbage\n", readPidsMaxFile(t, dir))
	})
}

func writeProcSys(t *testing.T, pidMax, threadsMax string) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "kernel"), 0o755))

	if pidMax != "" {
		require.NoError(t, os.WriteFile(filepath.Join(root, "kernel", "pid_max"), []byte(pidMax), 0o644))
	}

	if threadsMax != "" {
		require.NoError(t, os.WriteFile(filepath.Join(root, "kernel", "threads-max"), []byte(threadsMax), 0o644))
	}

	return root
}

func TestPidCapacity(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		pidMax     string
		threadsMax string
		expected   uint64
		errMsg     string
	}{
		{name: "pid_max lower", pidMax: "262144\n", threadsMax: "500000\n", expected: 262144},
		{name: "threads-max lower", pidMax: "262144\n", threadsMax: "15000\n", expected: 15000},
		{name: "equal", pidMax: "32768\n", threadsMax: "32768\n", expected: 32768},
		{name: "missing pid_max", threadsMax: "15000\n", errMsg: "pid_max"},
		{name: "missing threads-max", pidMax: "262144\n", errMsg: "threads-max"},
		{name: "malformed pid_max", pidMax: "lots\n", threadsMax: "15000\n", errMsg: "error parsing"},
		{name: "malformed threads-max", pidMax: "262144\n", threadsMax: "-1\n", errMsg: "error parsing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			capacity, err := cgroup.PidCapacity(writeProcSys(t, test.pidMax, test.threadsMax))

			if test.errMsg != "" {
				require.ErrorContains(t, err, test.errMsg)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, capacity)
		})
	}
}

func TestAllocatablePids(t *testing.T) {
	t.Parallel()

	assert.Equal(t, uint64(262044), cgroup.AllocatablePids(262144, 100))
	assert.Equal(t, uint64(1), cgroup.AllocatablePids(101, 100))
	assert.Equal(t, uint64(0), cgroup.AllocatablePids(100, 100))
	assert.Equal(t, uint64(0), cgroup.AllocatablePids(20, 100))
}
