// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerpid_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/lifecycle/internal/containerpid"
)

const cgroupPath = "system/installer"

// fakeProcfs builds a cgroupfs/procfs pair: the container cgroup lists procs, and each
// entry of statuses becomes a /proc/<pid>/status file.
func fakeProcfs(t *testing.T, procs string, statuses map[int32]string) *containerpid.Resolver {
	t.Helper()

	root := t.TempDir()

	cgroupMountPath := filepath.Join(root, "cgroup")
	procPath := filepath.Join(root, "proc")

	require.NoError(t, os.MkdirAll(filepath.Join(cgroupMountPath, cgroupPath), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cgroupMountPath, cgroupPath, "cgroup.procs"), []byte(procs), 0o644))

	for pid, status := range statuses {
		procDir := filepath.Join(procPath, strconv.Itoa(int(pid)))

		require.NoError(t, os.MkdirAll(procDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(procDir, "status"), []byte(status), 0o644))
	}

	return containerpid.NewResolverWithPaths(cgroupMountPath, procPath)
}

// status renders a /proc/<pid>/status stub with the given NSpid entries.
func status(nsPIDs ...int32) string {
	entries := make([]string, 0, len(nsPIDs))

	for _, nsPID := range nsPIDs {
		entries = append(entries, strconv.Itoa(int(nsPID)))
	}

	return "Name:\tinstaller\nState:\tS (sleeping)\nNSpid:\t" + strings.Join(entries, "\t") + "\nNSpgid:\t1\n"
}

func TestResolve(t *testing.T) {
	// the container init and two of its children, as seen by machined (4242...) and by
	// the containerd running in the sandbox PID namespace (17...)
	sandboxed := map[int32]string{
		4242: status(4242, 16),
		4243: status(4243, 17),
		4244: status(4244, 18),
	}

	for _, test := range []struct {
		name     string
		procs    string
		statuses map[int32]string
		nsPID    uint32
		expected int32
	}{
		{
			name:     "sandboxed container init",
			procs:    "4242\n4243\n4244\n",
			statuses: sandboxed,
			nsPID:    17,
			expected: 4243,
		},
		{
			name:     "single process in the cgroup",
			procs:    "4243\n",
			statuses: sandboxed,
			nsPID:    17,
			expected: 4243,
		},
		{
			name:  "no PID namespace nesting",
			procs: "4242\n4243\n",
			statuses: map[int32]string{
				4242: status(4242),
				4243: status(4243),
			},
			nsPID:    4243,
			expected: 4243,
		},
		{
			name:     "process exited while scanning",
			procs:    "4241\n4243\n",
			statuses: sandboxed, // no status for 4241: it is gone, and skipped
			nsPID:    17,
			expected: 4243,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pid, err := fakeProcfs(t, test.procs, test.statuses).Resolve(cgroupPath, test.nsPID)

			require.NoError(t, err)
			assert.Equal(t, test.expected, pid)
		})
	}
}

func TestResolveGone(t *testing.T) {
	statuses := map[int32]string{
		4242: status(4242, 16),
		4244: status(4244, 18),
	}

	for _, test := range []struct {
		name     string
		procs    string
		statuses map[int32]string
	}{
		{
			name:     "init exited, children linger",
			procs:    "4242\n4244\n",
			statuses: statuses,
		},
		{
			name:     "whole cgroup exited",
			procs:    "4242\n4243\n4244\n",
			statuses: nil,
		},
		{
			name:     "empty cgroup",
			procs:    "",
			statuses: statuses,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := fakeProcfs(t, test.procs, test.statuses).Resolve(cgroupPath, 17)

			assert.ErrorIs(t, err, containerpid.ErrGone)
		})
	}
}

func TestResolveError(t *testing.T) {
	for _, test := range []struct {
		name        string
		procs       string
		statuses    map[int32]string
		expectedErr string
	}{
		{
			name:        "malformed cgroup.procs",
			procs:       "4243\nnot-a-pid\n",
			statuses:    map[int32]string{4243: status(4243, 16)},
			expectedErr: `failed to parse PID "not-a-pid"`,
		},
		{
			name:        "status without NSpid",
			procs:       "4243\n",
			statuses:    map[int32]string{4243: "Name:\tinstaller\nNSpgid:\t1\n"},
			expectedErr: "no NSpid field found",
		},
		{
			name:        "status with empty NSpid",
			procs:       "4243\n",
			statuses:    map[int32]string{4243: "Name:\tinstaller\nNSpid:\t\n"},
			expectedErr: "NSpid field is empty",
		},
		{
			name:        "status with malformed NSpid",
			procs:       "4243\n",
			statuses:    map[int32]string{4243: "Name:\tinstaller\nNSpid:\t4243\txyz\n"},
			expectedErr: `failed to parse NSpid entry "xyz"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := fakeProcfs(t, test.procs, test.statuses).Resolve(cgroupPath, 17)

			assert.ErrorContains(t, err, test.expectedErr)
			// a malformed procfs/cgroupfs is a real problem, not a process which raced away
			assert.NotErrorIs(t, err, containerpid.ErrGone)
		})
	}
}

func TestResolveMissingCgroup(t *testing.T) {
	resolver := fakeProcfs(t, "", nil)

	_, err := resolver.Resolve("system/no-such-container", 17)

	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.NotErrorIs(t, err, containerpid.ErrGone)
}

// TestResolveLive resolves the PID of the test process itself via the cgroup it runs in.
func TestResolveLive(t *testing.T) {
	selfCgroup, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Skip("no procfs available")
	}

	// cgroupv2 lists a single "0::<path>" entry
	_, path, ok := strings.Cut(strings.TrimSpace(string(selfCgroup)), "0::")
	if !ok {
		t.Skip("not running under cgroupv2")
	}

	if _, err = os.Stat(filepath.Join("/sys/fs/cgroup", path, "cgroup.procs")); err != nil {
		t.Skip("own cgroup is not readable under /sys/fs/cgroup")
	}

	// the test process is read from its own PID namespace, so its innermost PID is the
	// one os.Getpid() reports
	pid, err := containerpid.NewResolver().Resolve(path, uint32(os.Getpid()))

	require.NoError(t, err)
	assert.Equal(t, int32(os.Getpid()), pid)
}
