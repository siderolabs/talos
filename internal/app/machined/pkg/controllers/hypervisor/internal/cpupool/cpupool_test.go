// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cpupool_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/cpupool"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var (
	sharedPool   = runtime.CPUPartitionPoolSpec{Name: "shared", CPUs: "4", Partition: "/virtualmachines.partition/shared.partition"}
	batchPool    = runtime.CPUPartitionPoolSpec{Name: "batch", CPUs: "5", Partition: "/virtualmachines.partition/batch.partition"}
	databasePool = runtime.CPUPartitionPoolSpec{Name: "database", CPUs: "6-7", Partition: "/virtualmachines.partition/database.partition"}
)

func TestSelect(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		pools []runtime.CPUPartitionPoolSpec
		cpu   hypervisor.VirtualMachineCPUSpec

		partition string
		cpus      string
		err       string
	}{
		{
			name:      "no pools",
			cpu:       hypervisor.VirtualMachineCPUSpec{Count: 2, EmulatorPin: "0"},
			partition: "/virtualmachines.partition",
		},
		{
			name: "slice without pools",
			cpu:  hypervisor.VirtualMachineCPUSpec{Slice: "database"},
			err:  `CPU slice "database" is not declared`,
		},
		{
			name:      "shared pool",
			pools:     []runtime.CPUPartitionPoolSpec{sharedPool, batchPool, databasePool},
			partition: sharedPool.Partition,
			cpus:      "4",
		},
		{
			name:      "named slice",
			pools:     []runtime.CPUPartitionPoolSpec{sharedPool, batchPool, databasePool},
			cpu:       hypervisor.VirtualMachineCPUSpec{Slice: "database", Pins: []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "6"}}, EmulatorPin: "7"},
			partition: databasePool.Partition,
			cpus:      "6-7",
		},
		{
			name:  "undeclared slice",
			pools: []runtime.CPUPartitionPoolSpec{sharedPool, databasePool},
			cpu:   hypervisor.VirtualMachineCPUSpec{Slice: "web"},
			err:   `CPU slice "web" is not declared`,
		},
		{
			name:  "fully carved root has no shared pool",
			pools: []runtime.CPUPartitionPoolSpec{databasePool},
			err:   "the named slices take every CPU of the virtual machine root",
		},
		{
			name:  "vCPU pin outside the slice",
			pools: []runtime.CPUPartitionPoolSpec{sharedPool, batchPool, databasePool},
			cpu:   hypervisor.VirtualMachineCPUSpec{Slice: "batch", Pins: []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "5-6"}}},
			err:   `vCPU 0 pin names host CPUs "6" outside the CPUs "5" of its pool`,
		},
		{
			name:  "emulator pin outside the shared pool",
			pools: []runtime.CPUPartitionPoolSpec{sharedPool, batchPool, databasePool},
			cpu:   hypervisor.VirtualMachineCPUSpec{EmulatorPin: "0"},
			err:   `emulator pin names host CPUs "0" outside the CPUs "4" of its pool`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			pool, err := cpupool.Select("vm", test.pools, test.cpu)

			if test.err != "" {
				require.ErrorIs(t, err, cpupool.ErrUnavailable)
				assert.ErrorContains(t, err, `virtual machine "vm": CPU placement unavailable: `+test.err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.partition, pool.Partition)
			assert.Equal(t, test.cpus, pool.CPUs.String())
		})
	}
}

func TestResolve(t *testing.T) {
	t.Parallel()

	for _, status := range []*runtime.CPUPartitionStatusSpec{nil, {VirtualMachinePools: []runtime.CPUPartitionPoolSpec{sharedPool}}} {
		_, err := cpupool.Resolve("vm", status, hypervisor.VirtualMachineCPUSpec{})
		require.ErrorIs(t, err, cpupool.ErrPending)
		assert.True(t, cpupool.IsHeld(err))
		assert.EqualError(t, err, `virtual machine "vm": CPU partition policy of the boot is not applied yet`)
	}

	pool, err := cpupool.Resolve("vm", &runtime.CPUPartitionStatusSpec{Ready: true, VirtualMachinePools: []runtime.CPUPartitionPoolSpec{sharedPool}}, hypervisor.VirtualMachineCPUSpec{})
	require.NoError(t, err)
	assert.Equal(t, sharedPool.Partition, pool.Partition)

	_, err = cpupool.Resolve("vm", &runtime.CPUPartitionStatusSpec{Ready: true}, hypervisor.VirtualMachineCPUSpec{Slice: "web"})
	assert.True(t, cpupool.IsHeld(err))
	assert.False(t, cpupool.IsHeld(errors.New("other")))
}
