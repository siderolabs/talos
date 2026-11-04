// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupartition_test

import (
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/cpupartition"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var online = cpuset.New(0, 1, 2, 3, 4, 5, 6, 7)

func examplePolicy(slices ...runtime.CPUPartitionSliceSpec) *runtime.CPUPartitionSpecSpec {
	return &runtime.CPUPartitionSpecSpec{
		Roots: map[string]string{
			"init":            "0-1",
			"system":          "0-1",
			"podruntime":      "0-1",
			"kubepods":        "2-3",
			"taloscontainers": "0-1",
			"virtualmachines": "4-7",
		},
		Slices: slices,
	}
}

func paths(policy cpupartition.Policy) []string {
	return xslices.Map(policy.Masks, func(mask cpupartition.Mask) string { return mask.Target.CgroupPath() })
}

func TestNewPolicyOrdersParentsFirst(t *testing.T) {
	t.Parallel()

	policy, err := cpupartition.NewPolicy(examplePolicy(
		runtime.CPUPartitionSliceSpec{Name: "database", CPUs: "6-7", Exclusive: true},
		runtime.CPUPartitionSliceSpec{Name: "batch", CPUs: "5"},
	), online)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"init", "system", "podruntime", "kubepods", "taloscontainers", "virtualmachines.partition",
		"virtualmachines.partition/shared.partition",
		"virtualmachines.partition/batch.partition",
		"virtualmachines.partition/database.partition",
	}, paths(policy), "the virtual machine root, then the shared pool and plain slices, exclusive slices last")

	assert.True(t, policy.Partitioned)

	vmRoot := policy.Masks[5]
	assert.Equal(t, cpuset.New(6, 7), vmRoot.ExclusiveCPUs, "the root delegates the exclusive CPUs")
	assert.Equal(t, cpuset.New(4, 5), vmRoot.Effective(), "the root keeps the CPUs its exclusive slices do not take")

	database := policy.Masks[8]
	assert.True(t, database.Exclusive)
	assert.Equal(t, cpuset.New(6, 7), database.ExclusiveCPUs)
	assert.Equal(t, cpuset.New(6, 7), database.Effective())

	for _, mask := range policy.Masks[:5] {
		assert.True(t, mask.ExclusiveCPUs.IsEmpty(), mask.Target.String())
	}

	assert.Equal(t, runtime.CPUPartitionStatusSpec{
		Ready:               true,
		KubeletReservedCPUs: "0-1,4-7",
		VirtualMachinePools: []runtime.CPUPartitionPoolSpec{
			{Name: "shared", CPUs: "4", Partition: "/virtualmachines.partition/shared.partition"},
			{Name: "batch", CPUs: "5", Partition: "/virtualmachines.partition/batch.partition"},
			{Name: "database", CPUs: "6-7", Partition: "/virtualmachines.partition/database.partition"},
		},
	}, policy.Status())
}

func TestNewPolicyFullyCarvedRootHasNoSharedPool(t *testing.T) {
	t.Parallel()

	policy, err := cpupartition.NewPolicy(examplePolicy(
		runtime.CPUPartitionSliceSpec{Name: "batch", CPUs: "4-5"},
		runtime.CPUPartitionSliceSpec{Name: "web", CPUs: "6-7"},
	), online)
	require.NoError(t, err)

	assert.NotContains(t, paths(policy), "virtualmachines.partition/shared.partition", "a machine without a slice must not fall back to the whole root")
	assert.False(t, policy.Partitioned)
	assert.Equal(t, []string{"batch", "web"}, xslices.Map(policy.Status().VirtualMachinePools, func(p runtime.CPUPartitionPoolSpec) string { return p.Name }))
}

func TestNewPolicyWithoutVirtualMachines(t *testing.T) {
	t.Parallel()

	policy, err := cpupartition.NewPolicy(&runtime.CPUPartitionSpecSpec{Roots: map[string]string{"system": "0-1", "kubepods": "2-7"}}, online)
	require.NoError(t, err)

	assert.Equal(t, []string{"system", "kubepods"}, paths(policy))
	assert.Equal(t, runtime.CPUPartitionStatusSpec{Ready: true, KubeletReservedCPUs: "0-1"}, policy.Status())

	policy, err = cpupartition.NewPolicy(&runtime.CPUPartitionSpecSpec{Roots: map[string]string{"system": "0-1"}}, online)
	require.NoError(t, err)
	assert.Equal(t, runtime.CPUPartitionStatusSpec{Ready: true}, policy.Status(), "unbounded kubepods reserves nothing")
}

func TestNewPolicyRejects(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		spec *runtime.CPUPartitionSpecSpec
		err  string
	}{
		{
			name: "kubepods takes every online CPU",
			spec: &runtime.CPUPartitionSpecSpec{Roots: map[string]string{"kubepods": "0-7"}},
			err:  `root kubepods: CPUs "0-7" leave no online CPU outside kubepods`,
		},
		{
			name: "offline CPUs",
			spec: &runtime.CPUPartitionSpecSpec{Roots: map[string]string{"system": "0-9"}},
			err:  `root system: CPUs "8-9" are offline`,
		},
		{
			name: "invalid list",
			spec: &runtime.CPUPartitionSpecSpec{Roots: map[string]string{"system": "1-"}},
			err:  `root system: invalid CPU list "1-"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := cpupartition.NewPolicy(test.spec, online)
			assert.ErrorContains(t, err, test.err)
		})
	}
}
