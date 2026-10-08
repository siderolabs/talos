// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
)

const delegation = "+cpu +cpuset"

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(data)
}

// newHostFS returns a HostFS over a temporary tree with a cgroup root delegating nothing and an
// existing virtual machine root no slice was ever created under.
func newHostFS(t *testing.T) (cpupartition.HostFS, string) {
	t.Helper()

	root := t.TempDir()

	writeFile(t, filepath.Join(root, "cgroup.subtree_control"), "")
	writeFile(t, filepath.Join(root, "virtualmachines.partition", "cgroup.subtree_control"), "")

	return cpupartition.HostFS{CgroupRoot: root, SysfsPath: filepath.Join(root, "sys")}, root
}

func TestHostFSEnsureDelegatesOnUnusedParents(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)
	vmRoot := filepath.Join(root, "virtualmachines.partition")

	require.NoError(t, hostFS.Ensure(cpupartition.Shared.CgroupPath()))

	assert.DirExists(t, filepath.Join(vmRoot, "shared.partition"))
	assert.Equal(t, delegation, readFile(t, filepath.Join(root, "cgroup.subtree_control")))
	assert.Equal(t, delegation, readFile(t, filepath.Join(vmRoot, "cgroup.subtree_control")))
	assert.NoFileExists(t, filepath.Join(vmRoot, "shared.partition", "cgroup.subtree_control"), "the target itself delegates nothing")

	// Existing cgroups are delegated again: a parent may have lost cpuset since.
	writeFile(t, filepath.Join(vmRoot, "cgroup.subtree_control"), "")
	require.NoError(t, hostFS.Ensure(cpupartition.Shared.CgroupPath()))
	assert.Equal(t, delegation, readFile(t, filepath.Join(vmRoot, "cgroup.subtree_control")))
}

func TestHostFSEnsureRoot(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)

	require.NoError(t, hostFS.Ensure("system"))

	assert.DirExists(t, filepath.Join(root, "system"))
	assert.Equal(t, delegation, readFile(t, filepath.Join(root, "cgroup.subtree_control")))
	assert.Empty(t, readFile(t, filepath.Join(root, "virtualmachines.partition", "cgroup.subtree_control")))
}

// TestHostFSEnsureDelegationFailure checks that a child is never created under a parent which
// could not delegate, and that ancestors are delegated top-down.
func TestHostFSEnsureDelegationFailure(t *testing.T) {
	t.Parallel()

	t.Run("parent", func(t *testing.T) {
		t.Parallel()

		hostFS, root := newHostFS(t)
		vmRoot := filepath.Join(root, "virtualmachines.partition")

		require.NoError(t, os.Remove(filepath.Join(vmRoot, "cgroup.subtree_control")))

		err := hostFS.Ensure(cpupartition.Slice("database").CgroupPath())
		require.ErrorIs(t, err, fs.ErrNotExist)
		assert.ErrorContains(t, err, vmRoot)

		assert.NoDirExists(t, filepath.Join(vmRoot, "database.partition"))
		assert.NoFileExists(t, filepath.Join(vmRoot, "cgroup.subtree_control"), "delegation never creates the control file")
		assert.Equal(t, delegation, readFile(t, filepath.Join(root, "cgroup.subtree_control")))
	})

	t.Run("root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		hostFS := cpupartition.HostFS{CgroupRoot: root}

		require.ErrorIs(t, hostFS.Ensure(cpupartition.Shared.CgroupPath()), fs.ErrNotExist)
		assert.NoDirExists(t, filepath.Join(root, "virtualmachines.partition"))
	})

	t.Run("write", func(t *testing.T) {
		t.Parallel()

		hostFS, root := newHostFS(t)
		control := filepath.Join(root, "virtualmachines.partition", "cgroup.subtree_control")

		require.NoError(t, os.Remove(control))
		require.NoError(t, os.Mkdir(control, 0o755))

		require.Error(t, hostFS.Ensure(cpupartition.Shared.CgroupPath()))
		assert.NoDirExists(t, filepath.Join(root, "virtualmachines.partition", "shared.partition"))
	})
}

func TestHostFSEnsureInvalidPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"",
		"/virtualmachines.partition",
		"virtualmachines.partition/../kubepods",
		"virtualmachines.partition//shared.partition",
		"virtualmachines.partition/",
	} {
		hostFS, root := newHostFS(t)

		require.Error(t, hostFS.Ensure(path), path)

		entries, err := os.ReadDir(root)
		require.NoError(t, err)
		assert.Len(t, entries, 2, path)
		assert.Empty(t, readFile(t, filepath.Join(root, "cgroup.subtree_control")), path)
	}
}

func TestHostFSMasks(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)
	slice := filepath.Join(root, "virtualmachines.partition", "database.partition")

	writeFile(t, filepath.Join(slice, "cpuset.cpus"), "\n")
	writeFile(t, filepath.Join(slice, "cpuset.cpus.effective"), "4-7\n")

	path := cpupartition.Slice("database").CgroupPath()

	configured, err := hostFS.CPUs(path)
	require.NoError(t, err)
	assert.Empty(t, configured, "an inheriting cgroup has no configured mask")

	effective, err := hostFS.Effective(path)
	require.NoError(t, err)
	assert.Equal(t, cpuset.New(4, 5, 6, 7), effective)

	require.NoError(t, hostFS.SetCPUs(path, "4-5"))
	assert.Equal(t, "4-5\n", readFile(t, filepath.Join(slice, "cpuset.cpus")))

	configured, err = hostFS.CPUs(path)
	require.NoError(t, err)
	assert.Equal(t, "4-5", configured)

	require.NoError(t, hostFS.SetCPUs(path, ""))
	assert.Equal(t, "\n", readFile(t, filepath.Join(slice, "cpuset.cpus")), "an empty write restores inheritance")

	writeFile(t, filepath.Join(slice, "cpuset.cpus.effective"), "4-\n")

	_, err = hostFS.Effective(path)
	assert.Error(t, err)
}

func TestHostFSMasksWithoutCpuset(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)
	slice := filepath.Join(root, "virtualmachines.partition", "database.partition")

	require.NoError(t, os.Mkdir(slice, 0o755))

	path := cpupartition.Slice("database").CgroupPath()

	require.ErrorIs(t, hostFS.SetCPUs(path, "4-5"), fs.ErrNotExist)
	assert.NoFileExists(t, filepath.Join(slice, "cpuset.cpus"), "a write never creates the cpuset interface")

	_, err := hostFS.CPUs(path)
	require.ErrorIs(t, err, fs.ErrNotExist)

	_, err = hostFS.Effective(path)
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestHostFSPopulated(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		events    string
		populated bool
		err       string
	}{
		{name: "populated", events: "populated 1\nfrozen 0\n", populated: true},
		{name: "empty", events: "populated 0\nfrozen 0\n"},
		{name: "missing entry", events: "frozen 0\n", err: `cgroup.events of "kubepods" has no populated entry`},
		{name: "malformed", events: "populated\n", err: "invalid format"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			hostFS, root := newHostFS(t)

			writeFile(t, filepath.Join(root, "kubepods", "cgroup.events"), test.events)

			populated, err := hostFS.Populated("kubepods")

			if test.err != "" {
				require.ErrorContains(t, err, test.err)
				assert.False(t, populated)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.populated, populated)
		})
	}

	hostFS, _ := newHostFS(t)

	_, err := hostFS.Populated("kubepods")
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func TestHostFSLeafEffective(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)
	kubepods := filepath.Join(root, "kubepods")

	cgroup := func(path, configured, effective string) {
		writeFile(t, filepath.Join(kubepods, path, "cpuset.cpus"), configured+"\n")
		writeFile(t, filepath.Join(kubepods, path, "cpuset.cpus.effective"), effective+"\n")
		writeFile(t, filepath.Join(kubepods, path, "cgroup.events"), "populated 1\n")
	}

	writeFile(t, filepath.Join(kubepods, "cpuset.cpus"), "2-5\n")
	cgroup("burstable", "", "2-5")
	cgroup("burstable/pod-a", "", "2-5")
	cgroup("burstable/pod-a/shared", "", "2-5")
	cgroup("pod-b", "4-5", "4-5")
	cgroup("pod-b/pinned", "4", "4")
	cgroup("pod-b/sandbox", "2-5", "2-3,5")
	cgroup("besteffort", "", "2-5")

	leaves, err := hostFS.LeafEffective("kubepods")
	require.NoError(t, err)
	assert.Equal(t, map[string]cpuset.CPUSet{
		"pod-b/pinned":  cpuset.New(4),
		"pod-b/sandbox": cpuset.New(2, 3, 5),
	}, leaves, "only leaves with a configured mask hold CPUs")

	leaves, err = hostFS.LeafEffective("taloscontainers")
	require.NoError(t, err)
	assert.Empty(t, leaves, "a missing parent has no leaves")

	writeFile(t, filepath.Join(kubepods, "pod-b", "sandbox", "cpuset.cpus.effective"), "garbage\n")

	_, err = hostFS.LeafEffective("kubepods")
	require.Error(t, err)

	require.NoError(t, os.Remove(filepath.Join(kubepods, "pod-b", "sandbox", "cpuset.cpus.effective")))

	_, err = hostFS.LeafEffective("kubepods")
	require.ErrorIs(t, err, fs.ErrNotExist)

	require.NoError(t, os.Remove(filepath.Join(kubepods, "pod-b", "sandbox", "cpuset.cpus")))

	_, err = hostFS.LeafEffective("kubepods")
	require.ErrorIs(t, err, fs.ErrNotExist, "an unreadable leaf is never treated as released")
}

func TestHostFSLifecycle(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)
	path := cpupartition.Slice("database").CgroupPath()

	exists, err := hostFS.Exists(path)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, hostFS.Ensure(path))

	exists, err = hostFS.Exists(path)
	require.NoError(t, err)
	assert.True(t, exists)

	writeFile(t, filepath.Join(root, path, "child", "cgroup.events"), "populated 0\n")
	require.Error(t, hostFS.Remove(path), "a cgroup with children is not removed")

	require.NoError(t, os.RemoveAll(filepath.Join(root, path, "child")))
	require.NoError(t, hostFS.Remove(path))

	exists, err = hostFS.Exists(path)
	require.NoError(t, err)
	assert.False(t, exists)

	writeFile(t, filepath.Join(root, "kubepods"), "")

	_, err = hostFS.Exists("kubepods/burstable")
	assert.Error(t, err, "errors other than a missing cgroup are reported")
}

func TestHostFSOnline(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)

	_, err := hostFS.Online()
	require.ErrorIs(t, err, fs.ErrNotExist)

	writeFile(t, filepath.Join(root, "sys", "devices", "system", "cpu", "online"), "0-3,6\n")

	online, err := hostFS.Online()
	require.NoError(t, err)
	assert.Equal(t, cpuset.New(0, 1, 2, 3, 6), online)

	writeFile(t, filepath.Join(root, "sys", "devices", "system", "cpu", "online"), "0-\n")

	_, err = hostFS.Online()
	assert.Error(t, err)
}

func TestHostFSChildren(t *testing.T) {
	t.Parallel()

	hostFS, root := newHostFS(t)
	vmRoot := filepath.Join(root, "virtualmachines.partition")

	require.NoError(t, os.MkdirAll(filepath.Join(vmRoot, "shared.partition"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(vmRoot, "machine-qemu-legacy.scope", "vcpu0"), 0o755))
	writeFile(t, filepath.Join(vmRoot, "cgroup.events"), "populated 1\n")

	children, err := hostFS.Children("virtualmachines.partition")
	require.NoError(t, err)
	assert.Equal(t, []string{"machine-qemu-legacy.scope", "shared.partition"}, children, "direct child cgroups only, no interface files")

	children, err = hostFS.Children("virtualmachines.partition/missing.partition")
	require.NoError(t, err)
	assert.Empty(t, children, "a missing cgroup has no children")

	writeFile(t, filepath.Join(root, "notadir"), "")

	_, err = hostFS.Children("notadir")
	require.Error(t, err, "a read failure is not an empty cgroup")
}
