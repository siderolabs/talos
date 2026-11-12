// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerpid_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/containerpid"
)

const cgroupPath = "system/installer"

// PID namespace inodes of the fake process tree.
const (
	rootNS      uint64 = 4026531836 // machined
	sandboxNS   uint64 = 4026532001 // the CRI containerd under sandboxd
	containerNS uint64 = 4026532002 // a container with a PID namespace of its own
	otherNS     uint64 = 4026532003 // unrelated to the runtime
)

// proc describes a fake process: its NSpid entries and its PID namespace chain, innermost first.
//
// A process with nsErr set fails the read of its PID namespaces with it.
type proc struct {
	nsPIDs     []int32
	namespaces []uint64
	nsErr      error
}

// fakeProcfs builds a cgroupfs/procfs pair: the container cgroup lists cgroupProcs, and each
// entry of procs becomes a /proc/<pid>/status file and a PID namespace chain. statuses replaces
// the status file rendered for a process.
func fakeProcfs(t *testing.T, cgroupProcs string, procs map[int32]proc, statuses map[int32]string) *containerpid.Resolver {
	t.Helper()

	return fakeProcfsCgroups(t, map[string]string{cgroupPath: cgroupProcs}, procs, statuses)
}

// fakeProcfsCgroups is fakeProcfs with any number of cgroups: cgroups maps a cgroup path, relative
// to the cgroupfs mount point, to the contents of its cgroup.procs.
func fakeProcfsCgroups(t *testing.T, cgroups map[string]string, procs map[int32]proc, statuses map[int32]string) *containerpid.Resolver {
	t.Helper()

	root := t.TempDir()

	cgroupMountPath := filepath.Join(root, "cgroup")
	procPath := filepath.Join(root, "proc")

	for path, cgroupProcs := range cgroups {
		require.NoError(t, os.MkdirAll(filepath.Join(cgroupMountPath, path), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(cgroupMountPath, path, "cgroup.procs"), []byte(cgroupProcs), 0o644))
	}

	for pid, p := range procs {
		procDir := filepath.Join(procPath, strconv.Itoa(int(pid)))

		require.NoError(t, os.MkdirAll(procDir, 0o755))

		contents, ok := statuses[pid]
		if !ok {
			contents = status(p.nsPIDs...)
		}

		require.NoError(t, os.WriteFile(filepath.Join(procDir, "status"), []byte(contents), 0o644))
	}

	return containerpid.NewResolverWithPaths(cgroupMountPath, procPath, func(pid string) ([]uint64, error) {
		if pid == "self" {
			return []uint64{rootNS}, nil
		}

		n, err := strconv.Atoi(pid)
		if err != nil {
			return nil, err
		}

		p, ok := procs[int32(n)]
		if ok && p.nsErr != nil {
			return nil, p.nsErr
		}

		if !ok || p.namespaces == nil {
			return nil, fmt.Errorf("process %d: %w", n, fs.ErrNotExist)
		}

		return p.namespaces, nil
	})
}

// status renders a /proc/<pid>/status stub with the given NSpid entries.
func status(nsPIDs ...int32) string {
	entries := make([]string, 0, len(nsPIDs))

	for _, nsPID := range nsPIDs {
		entries = append(entries, strconv.Itoa(int(nsPID)))
	}

	return "Name:\tinstaller\nState:\tS (sleeping)\nNSpid:\t" + strings.Join(entries, "\t") + "\nNSpgid:\t1\n"
}

// sharedNS is a process sharing the PID namespace of the sandboxed runtime.
func sharedNS(hostPID, sandboxPID int32) proc {
	return proc{nsPIDs: []int32{hostPID, sandboxPID}, namespaces: []uint64{sandboxNS, rootNS}}
}

// ownNS is a process in a PID namespace of its own below the sandboxed runtime.
func ownNS(hostPID, sandboxPID, containerPID int32) proc {
	return proc{nsPIDs: []int32{hostPID, sandboxPID, containerPID}, namespaces: []uint64{containerNS, sandboxNS, rootNS}}
}

func TestResolve(t *testing.T) {
	for _, test := range []struct {
		name         string
		cgroupProcs  string
		procs        map[int32]proc
		runtimePIDNS uint64
		runtimePID   uint32
		expected     int32
	}{
		{
			name:         "runtime shares the caller's PID namespace",
			cgroupProcs:  "", // not even looked at
			runtimePIDNS: rootNS,
			runtimePID:   4243,
			expected:     4243,
		},
		{
			name:        "sandboxed, container shares the runtime's PID namespace",
			cgroupProcs: "4242\n4243\n4244\n",
			procs: map[int32]proc{
				4242: sharedNS(4242, 16),
				4243: sharedNS(4243, 17),
				4244: sharedNS(4244, 18),
			},
			runtimePIDNS: sandboxNS,
			runtimePID:   17,
			expected:     4243,
		},
		{
			name:        "sandboxed, container has a PID namespace of its own",
			cgroupProcs: "4243\n4244\n",
			procs: map[int32]proc{
				4243: ownNS(4243, 17, 1),
				4244: ownNS(4244, 18, 2),
			},
			runtimePIDNS: sandboxNS,
			runtimePID:   17,
			expected:     4243,
		},
		{
			name:        "sandboxed, own PID namespace whose PIDs collide with the runtime's",
			cgroupProcs: "4242\n4243\n",
			procs: map[int32]proc{
				4242: ownNS(4242, 16, 17),
				4243: ownNS(4243, 17, 1),
			},
			runtimePIDNS: sandboxNS,
			runtimePID:   17,
			expected:     4243,
		},
		{
			name:        "processes outside the runtime's PID namespace are never matched",
			cgroupProcs: "17\n4240\n4243\n",
			procs: map[int32]proc{
				// host PID equal to the one containerd reported
				17: {nsPIDs: []int32{17}, namespaces: []uint64{rootNS}},
				// innermost PID equal to the one containerd reported, in an unrelated namespace
				4240: {nsPIDs: []int32{4240, 17}, namespaces: []uint64{otherNS, rootNS}},
				4243: ownNS(4243, 17, 1),
			},
			runtimePIDNS: sandboxNS,
			runtimePID:   17,
			expected:     4243,
		},
		{
			name:        "process exited while scanning",
			cgroupProcs: "4241\n4243\n",
			procs: map[int32]proc{
				// no entry for 4241: it is gone, and skipped
				4243: sharedNS(4243, 17),
			},
			runtimePIDNS: sandboxNS,
			runtimePID:   17,
			expected:     4243,
		},
		{
			name:        "processes which cannot be inspected do not hide the init",
			cgroupProcs: "4241\n4242\n4243\n",
			procs: map[int32]proc{
				// the same PID in the runtime's namespace as far as NSpid tells, namespaces not readable
				4241: {nsPIDs: []int32{4241, 17}, nsErr: fs.ErrPermission},
				// no NSpid to go by at all
				4242: {nsPIDs: nil, namespaces: []uint64{sandboxNS, rootNS}},
				4243: sharedNS(4243, 17),
			},
			runtimePIDNS: sandboxNS,
			runtimePID:   17,
			expected:     4243,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pid, err := fakeProcfs(t, test.cgroupProcs, test.procs, nil).Resolve(cgroupPath, test.runtimePIDNS, test.runtimePID)

			require.NoError(t, err)
			assert.Equal(t, test.expected, pid)
		})
	}
}

// TestResolveSubCgroup covers an init which moved itself into a cgroup below the container one.
func TestResolveSubCgroup(t *testing.T) {
	procs := map[int32]proc{
		4243: ownNS(4243, 17, 1),
		4244: ownNS(4244, 18, 2),
	}

	resolver := fakeProcfsCgroups(t, map[string]string{
		cgroupPath:                                "",
		filepath.Join(cgroupPath, "workers"):      "4244\n",
		filepath.Join(cgroupPath, "init.scope"):   "",
		filepath.Join(cgroupPath, "init.scope/a"): "4243\n",
		// not below the container cgroup, never looked at
		"system/other": "4243\n",
	}, procs, nil)

	pid, err := resolver.Resolve(cgroupPath, sandboxNS, 17)

	require.NoError(t, err)
	assert.Equal(t, int32(4243), pid)

	_, err = resolver.Resolve(filepath.Join(cgroupPath, "workers"), sandboxNS, 17)

	assert.ErrorIs(t, err, containerpid.ErrGone)
}

// TestResolveCgroupV1 covers the cgroup v1 layout, with a directory per controller hierarchy.
func TestResolveCgroupV1(t *testing.T) {
	resolver := fakeProcfsCgroups(t, map[string]string{
		filepath.Join("cpu", "unrelated"): "",
		filepath.Join("pids", cgroupPath): "4243\n",
	}, map[int32]proc{4243: sharedNS(4243, 17)}, nil)

	pid, err := resolver.Resolve(cgroupPath, sandboxNS, 17)

	require.NoError(t, err)
	assert.Equal(t, int32(4243), pid)
}

func TestResolveGone(t *testing.T) {
	children := map[int32]proc{
		4242: sharedNS(4242, 16),
		4244: sharedNS(4244, 18),
	}

	for _, test := range []struct {
		name        string
		cgroupProcs string
		procs       map[int32]proc
	}{
		{
			name:        "init exited, children linger",
			cgroupProcs: "4242\n4244\n",
			procs:       children,
		},
		{
			name:        "whole cgroup exited",
			cgroupProcs: "4242\n4243\n4244\n",
			procs:       nil,
		},
		{
			name:        "empty cgroup",
			cgroupProcs: "",
			procs:       children,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := fakeProcfs(t, test.cgroupProcs, test.procs, nil).Resolve(cgroupPath, sandboxNS, 17)

			assert.ErrorIs(t, err, containerpid.ErrGone)
		})
	}
}

func TestResolveError(t *testing.T) {
	for _, test := range []struct {
		name        string
		cgroupProcs string
		procs       map[int32]proc
		statuses    map[int32]string
		expectedErr string
	}{
		{
			name:        "malformed cgroup.procs",
			cgroupProcs: "4243\nnot-a-pid\n",
			procs:       map[int32]proc{4243: sharedNS(4243, 16)},
			expectedErr: `failed to parse PID "not-a-pid"`,
		},
		{
			name:        "status without NSpid",
			cgroupProcs: "4243\n",
			procs:       map[int32]proc{4243: sharedNS(4243, 17)},
			statuses:    map[int32]string{4243: "Name:\tinstaller\nNSpgid:\t1\n"},
			expectedErr: "no NSpid field found",
		},
		{
			name:        "status with empty NSpid",
			cgroupProcs: "4243\n",
			procs:       map[int32]proc{4243: sharedNS(4243, 17)},
			statuses:    map[int32]string{4243: "Name:\tinstaller\nNSpid:\t\n"},
			expectedErr: "NSpid field is empty",
		},
		{
			name:        "status with malformed NSpid",
			cgroupProcs: "4243\n",
			procs:       map[int32]proc{4243: sharedNS(4243, 17)},
			statuses:    map[int32]string{4243: "Name:\tinstaller\nNSpid:\t4243\txyz\n"},
			expectedErr: `failed to parse NSpid entry "xyz"`,
		},
		{
			name:        "NSpid does not match the PID namespace chain",
			cgroupProcs: "4243\n",
			procs:       map[int32]proc{4243: {nsPIDs: []int32{4243, 17, 1}, namespaces: []uint64{sandboxNS, rootNS}}},
			expectedErr: "nested in 2 PID namespaces, but NSpid has 3 entries",
		},
		{
			name:        "PID namespaces not readable",
			cgroupProcs: "4242\n4243\n",
			procs: map[int32]proc{
				4242: sharedNS(4242, 16),
				4243: {nsPIDs: []int32{4243, 17}, nsErr: fs.ErrPermission},
			},
			expectedErr: "1 could not be inspected: failed to read PID namespaces of 4243: permission denied",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := fakeProcfs(t, test.cgroupProcs, test.procs, test.statuses).Resolve(cgroupPath, sandboxNS, 17)

			assert.ErrorContains(t, err, test.expectedErr)
			// a malformed procfs/cgroupfs is a real problem, not a process which raced away
			assert.NotErrorIs(t, err, containerpid.ErrGone)
		})
	}
}

func TestResolveMissingCgroup(t *testing.T) {
	resolver := fakeProcfs(t, "", nil, nil)

	_, err := resolver.Resolve("system/no-such-container", sandboxNS, 17)

	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.NotErrorIs(t, err, containerpid.ErrGone)

	_, err = resolver.Resolve("", sandboxNS, 17)

	assert.ErrorContains(t, err, "its cgroup is not known")
}

// ownCgroup returns the cgroupv2 path of the test process, skipping the test if it is not readable.
func ownCgroup(t *testing.T) string {
	t.Helper()

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

	return path
}

func pidNamespaceInode(t *testing.T, pid string) uint64 {
	t.Helper()

	st, err := os.Stat(filepath.Join("/proc", pid, "ns", "pid"))
	require.NoError(t, err)

	return st.Sys().(*syscall.Stat_t).Ino //nolint:forcetypeassert
}

func TestReadPIDNamespacesLive(t *testing.T) {
	namespaces, err := containerpid.ReadPIDNamespaces("/proc/self/ns/pid")
	require.NoError(t, err)

	// the walk stops at the caller's own namespace
	assert.Equal(t, []uint64{pidNamespaceInode(t, "self")}, namespaces)
}

// TestResolveLive resolves the PID of the test process itself, as seen from its own PID namespace.
func TestResolveLive(t *testing.T) {
	pid, err := containerpid.NewResolver().Resolve(ownCgroup(t), pidNamespaceInode(t, "self"), uint32(os.Getpid()))

	require.NoError(t, err)
	assert.Equal(t, int32(os.Getpid()), pid)
}

// TestResolveLiveNested resolves the PID of a child in a nested PID namespace, as seen from that
// namespace (where it is PID 1), back into the PID namespace of the test.
func TestResolveLiveNested(t *testing.T) {
	path := ownCgroup(t)

	cmd := exec.CommandContext(t.Context(), "sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWPID,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}

	if err := cmd.Start(); err != nil {
		t.Skipf("cannot create a nested PID namespace: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill() //nolint:errcheck
		cmd.Wait()         //nolint:errcheck
	})

	childPID := strconv.Itoa(cmd.Process.Pid)

	namespaces, err := containerpid.ReadPIDNamespaces(filepath.Join("/proc", childPID, "ns", "pid"))
	require.NoError(t, err)
	assert.Equal(t, []uint64{pidNamespaceInode(t, childPID), pidNamespaceInode(t, "self")}, namespaces)

	pid, err := containerpid.NewResolver().Resolve(path, pidNamespaceInode(t, childPID), 1)
	if errors.Is(err, fs.ErrPermission) {
		t.Skipf("own cgroup has processes the test cannot inspect: %v", err)
	}

	require.NoError(t, err)
	assert.Equal(t, int32(cmd.Process.Pid), pid)
}
