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

	expected := map[config.CPUPartitionRoot]struct {
		path, key, partition string
	}{
		constants.CgroupInit:                {path: "init", key: "init"},
		constants.CgroupSystem:              {path: "system", key: "system"},
		constants.CgroupPodRuntimeRoot:      {path: "podruntime", key: "podruntime"},
		constants.CgroupKubepods:            {path: "kubepods", key: "kubepods"},
		constants.CgroupTalosContainersRoot: {path: "taloscontainers", key: "taloscontainers"},
		constants.CgroupVirtualMachinesRoot: {
			path:      "virtualmachines.partition",
			key:       "virtualmachines",
			partition: "/virtualmachines.partition",
		},
	}

	require.Len(t, config.CPUPartitionRoots(), len(expected))

	for _, root := range config.CPUPartitionRoots() {
		want, ok := expected[root]
		require.True(t, ok, root)

		target := cpupartition.Root(root)

		assert.Equal(t, cpupartition.KindRoot, target.Kind)
		assert.Equal(t, want.path, target.CgroupPath(), root)
		assert.Equal(t, want.key, target.Key(), root)
		assert.Equal(t, want.partition, target.Partition(), root)
		assert.Equal(t, "root "+string(root), target.String())

		parsed, ok := cpupartition.ParseKey(target.Key())
		require.True(t, ok, root)
		assert.Equal(t, target, parsed)
	}
}

func TestVirtualMachineTargets(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "shared.partition", cpupartition.SharedPartition)

	for _, test := range []struct {
		target                         cpupartition.Target
		path, key, partition, describe string
	}{
		{
			target:    cpupartition.Shared,
			path:      "virtualmachines.partition/shared.partition",
			key:       "virtualmachines/shared",
			partition: "/virtualmachines.partition/shared.partition",
			describe:  "shared slice",
		},
		{
			target:    cpupartition.Slice("database"),
			path:      "virtualmachines.partition/database.partition",
			key:       "virtualmachines/database",
			partition: "/virtualmachines.partition/database.partition",
			describe:  "slice database",
		},
	} {
		assert.Equal(t, test.path, test.target.CgroupPath())
		assert.Equal(t, test.key, test.target.Key())
		assert.Equal(t, test.partition, test.target.Partition())
		assert.Equal(t, test.describe, test.target.String())

		parsed, ok := cpupartition.ParseKey(test.key)
		require.True(t, ok, test.key)
		assert.Equal(t, test.target, parsed)
	}
}

func TestInvalidTargets(t *testing.T) {
	t.Parallel()

	for _, target := range []cpupartition.Target{
		cpupartition.Root("virtualMachines"),
		cpupartition.Root("kubepods/burstable"),
		cpupartition.Root(""),
		cpupartition.Slice(""),
		cpupartition.Slice("shared"),
		cpupartition.Slice("../kubepods"),
		cpupartition.Slice("a/b"),
		cpupartition.Slice("db.partition"),
		{Kind: cpupartition.KindSlice + 1, Name: "database"},
	} {
		assert.Empty(t, target.CgroupPath(), target)
		assert.Empty(t, target.Partition(), target)
	}

	for _, key := range []string{
		"",
		"nothing",
		"virtualMachines",
		"virtualMachines/database",
		"virtualmachines.partition",
		"virtualmachines/",
		"virtualmachines/../kubepods",
		"virtualmachines/a/b",
		"virtualmachines/db.partition",
	} {
		_, ok := cpupartition.ParseKey(key)
		assert.False(t, ok, key)
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
		target := cpupartition.Slice(slice.Name())
		paths = append(paths, target.CgroupPath())

		parsed, ok := cpupartition.ParseKey(target.Key())
		require.True(t, ok)
		assert.Equal(t, target, parsed)
	}

	assert.Equal(t, []string{
		"virtualmachines.partition/database.partition",
		"virtualmachines.partition/web-2.partition",
	}, paths)
}
