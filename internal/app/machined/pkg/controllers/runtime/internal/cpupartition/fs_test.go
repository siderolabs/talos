// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
)

// TestHostFSEnsureEnablesCpuset reproduces the live failure where a slice created under a virtual
// machine root which no libvirt domain had started in yet came up without cpuset.cpus: the parent's
// cgroup.subtree_control only ever received "+cpu". Ensure must delegate cpuset down to the slice
// itself; the fake tree records what is written to each ancestor's cgroup.subtree_control.
func TestHostFSEnsureEnablesCpuset(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	parent := filepath.Join(root, "virtualmachines.partition")

	require.NoError(t, os.MkdirAll(parent, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "cgroup.subtree_control"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(parent, "cgroup.subtree_control"), nil, 0o644))

	hostFS := cpupartition.HostFS{CgroupRoot: root, SysfsPath: filepath.Join(root, "sys")}

	require.NoError(t, hostFS.Ensure("virtualmachines.partition/shared.partition"))

	assert.DirExists(t, filepath.Join(parent, "shared.partition"))

	for _, dir := range []string{root, parent} {
		data, err := os.ReadFile(filepath.Join(dir, "cgroup.subtree_control"))
		require.NoError(t, err)

		assert.Contains(t, string(data), "+cpuset", "cpuset not delegated by %s", dir)
	}
}
