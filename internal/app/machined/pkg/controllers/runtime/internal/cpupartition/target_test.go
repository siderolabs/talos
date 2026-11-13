// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestRootTargets(t *testing.T) {
	t.Parallel()

	expected := map[config.CPUPartitionRoot]string{
		constants.CgroupInit:                "init",
		constants.CgroupSystem:              "system",
		constants.CgroupPodRuntimeRoot:      "podruntime",
		constants.CgroupKubepods:            "kubepods",
		constants.CgroupTalosContainersRoot: "taloscontainers",
		constants.CgroupVirtualMachinesRoot: "virtualmachines.partition",
	}

	require.Len(t, config.CPUPartitionRoots(), len(expected))

	for _, root := range config.CPUPartitionRoots() {
		path, ok := expected[root]
		require.True(t, ok, root)

		target := cpupartition.Root(root)

		assert.Equal(t, cpupartition.KindRoot, target.Kind)
		assert.Equal(t, path, target.CgroupPath(), root)
		assert.Equal(t, "root "+string(root), target.String())
	}
}

func TestVirtualMachineTargets(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		target         cpupartition.Target
		path, describe string
	}{
		{
			target:   cpupartition.Shared,
			path:     "virtualmachines.partition/shared.partition",
			describe: "slice shared",
		},
		{
			target:   cpupartition.Slice("database"),
			path:     "virtualmachines.partition/database.partition",
			describe: "slice database",
		},
	} {
		assert.Equal(t, test.path, test.target.CgroupPath())
		assert.Equal(t, test.describe, test.target.String())
	}
}

func TestInvalidTargets(t *testing.T) {
	t.Parallel()

	for _, target := range []cpupartition.Target{
		cpupartition.Root("virtualMachines"),
		cpupartition.Root("kubepods/burstable"),
		cpupartition.Root(""),
		cpupartition.Slice(""),
		cpupartition.Slice("../kubepods"),
		cpupartition.Slice("a/b"),
		cpupartition.Slice("db.partition"),
	} {
		assert.Empty(t, target.CgroupPath(), target)
	}
}

// TestConfiguredTargets maps a loaded document onto cgroups: every accepted root and slice has a path.
func TestConfiguredTargets(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: CPUPartitionConfig
init:
    cpus: 0-1
system:
    cpus: 0-1
podruntime:
    cpus: 0-1
kubepods:
    cpus: 2-3
taloscontainers:
    cpus: 0-1
virtualMachines:
    cpus: 4-7
    slices:
        - name: database
          cpus: 4-5
          exclusive: true
        - name: web-2
          cpus: 6
`))
	require.NoError(t, err)

	partition := provider.CPUPartitionConfig()
	require.NotNil(t, partition)
	require.Len(t, partition.Roots(), len(config.CPUPartitionRoots()))

	for root := range partition.Roots() {
		assert.NotEmpty(t, cpupartition.Root(root).CgroupPath(), root)
	}

	paths := make([]string, 0, len(partition.Slices()))

	for _, slice := range partition.Slices() {
		paths = append(paths, cpupartition.Slice(slice.Name()).CgroupPath())
	}

	assert.Equal(t, []string{
		"virtualmachines.partition/database.partition",
		"virtualmachines.partition/web-2.partition",
	}, paths)
}
