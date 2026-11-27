// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
)

// writeFakeCgroup creates a fake cgroup directory with the given cgroup.procs and cgroup.events contents.
func writeFakeCgroup(t *testing.T, dir, procs, events string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cgroup.procs"), []byte(procs), 0o644))

	if events != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cgroup.events"), []byte(events), 0o644))
	}
}

func TestListProcesses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	child := filepath.Join(root, "besteffort", "pod1")

	self := os.Getpid()

	writeFakeCgroup(t, root, "", "")
	writeFakeCgroup(t, filepath.Join(root, "besteffort"), "", "")
	writeFakeCgroup(t, child, strconv.Itoa(self)+"\n", "")

	processes, err := cgroup.ListProcesses(root)
	require.NoError(t, err)
	require.Len(t, processes, 1)

	assert.Equal(t, self, processes[0].PID)
	assert.Equal(t, child, processes[0].Cgroup)
	assert.NotEqual(t, "<unknown>", processes[0].Comm)
	assert.NotEmpty(t, processes[0].Cmdline)

	_, err = cgroup.ListProcesses(filepath.Join(root, "missing"))
	require.ErrorIs(t, err, os.ErrNotExist)

	writeFakeCgroup(t, filepath.Join(root, "broken"), "not-a-pid\n", "")

	_, err = cgroup.ListProcesses(root)
	require.ErrorContains(t, err, "error parsing")
}

func TestKillProcessesFake(t *testing.T) {
	t.Parallel()

	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		killed, err := cgroup.KillProcesses(t.Context(), filepath.Join(t.TempDir(), "missing"))
		require.NoError(t, err)
		assert.Empty(t, killed)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeFakeCgroup(t, dir, "", "populated 0\nfrozen 0\n")

		killed, err := cgroup.KillProcesses(t.Context(), dir)
		require.NoError(t, err)
		assert.Empty(t, killed)

		// nothing to kill, so cgroup.kill is not written
		assert.NoFileExists(t, filepath.Join(dir, "cgroup.kill"))
	})

	t.Run("exited", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// the process is listed, and the cgroup is reported empty after the kill
		writeFakeCgroup(t, dir, "1\n", "populated 0\nfrozen 0\n")

		killed, err := cgroup.KillProcesses(t.Context(), dir)
		require.NoError(t, err)
		require.Len(t, killed, 1)
		assert.Equal(t, 1, killed[0].PID)

		contents, err := os.ReadFile(filepath.Join(dir, "cgroup.kill"))
		require.NoError(t, err)
		assert.Equal(t, "1", string(contents))
	})

	t.Run("stuck", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeFakeCgroup(t, dir, "1\n", "populated 1\nfrozen 0\n")

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		killed, err := cgroup.KillProcesses(ctx, dir)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Len(t, killed, 1)
	})
}

// TestKillProcessesKernel exercises KillProcesses against a real cgroup v2 directory.
//
// It runs only when TALOS_CGROUP_KILL_TEST_DIR names a delegated, otherwise unused cgroup
// the test may create child cgroups in and start processes in.
func TestKillProcessesKernel(t *testing.T) {
	dir := os.Getenv("TALOS_CGROUP_KILL_TEST_DIR")
	if dir == "" {
		t.Skip("TALOS_CGROUP_KILL_TEST_DIR is not set")
	}

	child := filepath.Join(dir, "kill-test", "nested")
	require.NoError(t, os.MkdirAll(child, 0o755))

	t.Cleanup(func() {
		assert.NoError(t, os.Remove(child))
		assert.NoError(t, os.Remove(filepath.Dir(child)))
	})

	cgroupFD, err := os.Open(child)
	require.NoError(t, err)

	t.Cleanup(func() { cgroupFD.Close() }) //nolint:errcheck

	// a process ignoring SIGTERM, which forks a child, as a stuck workload would
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "trap '' TERM; sleep 3600 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		UseCgroupFD: true,
		CgroupFD:    int(cgroupFD.Fd()),
	}

	require.NoError(t, cmd.Start())

	waitErr := make(chan error, 1)

	go func() { waitErr <- cmd.Wait() }()

	// wait for the shell to fork sleep
	require.Eventually(t, func() bool {
		processes, err := cgroup.ListProcesses(dir)

		return err == nil && len(processes) == 2
	}, 5*time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	killed, err := cgroup.KillProcesses(ctx, filepath.Dir(child))
	require.NoError(t, err)
	require.Len(t, killed, 2)

	for _, process := range killed {
		assert.Equal(t, child, process.Cgroup)
	}

	remaining, err := cgroup.ListProcesses(dir)
	require.NoError(t, err)
	assert.Empty(t, remaining)

	require.Error(t, <-waitErr, "the shell must be killed")
}
