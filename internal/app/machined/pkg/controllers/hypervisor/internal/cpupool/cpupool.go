// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cpupool selects the CPU pool a virtual machine runs in under the CPU partition policy of the boot.
package cpupool

import (
	"errors"
	"fmt"

	"k8s.io/utils/cpuset"

	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ErrPending marks a virtual machine waiting for the CPU partition policy of the boot.
var ErrPending = errors.New("CPU partition policy of the boot is not applied yet")

// ErrUnavailable marks a CPU selection the CPU partition policy of the boot cannot satisfy;
// a changed policy only takes effect after a reboot.
var ErrUnavailable = errors.New("CPU placement unavailable")

// IsHeld reports a definition held back by the CPU partition policy of the boot.
func IsHeld(err error) bool {
	return errors.Is(err, ErrPending) || errors.Is(err, ErrUnavailable)
}

// Resolve selects the pool of a virtual machine from the CPU partition status, nil while there is none yet.
func Resolve(name string, status *runtime.CPUPartitionStatusSpec, cpu hypervisor.VirtualMachineCPUSpec) (Pool, error) {
	if status == nil || !status.Ready {
		return Pool{}, fmt.Errorf("virtual machine %q: %w", name, ErrPending)
	}

	return Select(name, status.VirtualMachinePools, cpu)
}

// Pool is where a virtual machine runs under the CPU partition policy of the boot.
type Pool struct {
	// Partition is the libvirt resource partition.
	Partition string
	// CPUs bounds the host CPU pins; empty when the virtual machine root is not partitioned.
	CPUs cpuset.CPUSet
}

var unmanaged = Pool{Partition: "/" + constants.CgroupVirtualMachines, CPUs: cpuset.New()}

// Select picks the pool of the selected slice, or the shared pool, and checks the host CPU pins
// against it. Without pools the virtual machine root is not partitioned.
func Select(name string, pools []runtime.CPUPartitionPoolSpec, cpu hypervisor.VirtualMachineCPUSpec) (Pool, error) {
	unavailable := func(err error) error {
		return fmt.Errorf("virtual machine %q: %w: %w", name, ErrUnavailable, err)
	}

	if len(pools) == 0 && cpu.Slice == "" {
		return unmanaged, nil
	}

	selection := cpu.Slice
	if selection == "" {
		selection = runtimecfg.CPUPartitionSharedSliceName
	}

	for _, pool := range pools {
		if pool.Name != selection {
			continue
		}

		cpus, err := cpuset.Parse(pool.CPUs)
		if err != nil {
			return Pool{}, unavailable(fmt.Errorf("invalid CPUs %q of pool %q: %w", pool.CPUs, pool.Name, err))
		}

		if err = checkPins(cpu, cpus); err != nil {
			return Pool{}, unavailable(err)
		}

		return Pool{Partition: pool.Partition, CPUs: cpus}, nil
	}

	if cpu.Slice == "" {
		return Pool{}, unavailable(errors.New("the named slices take every CPU of the virtual machine root, select one of them"))
	}

	return Pool{}, unavailable(fmt.Errorf("CPU slice %q is not declared by the CPU partition policy of the boot", cpu.Slice))
}

func checkPins(cpu hypervisor.VirtualMachineCPUSpec, pool cpuset.CPUSet) error {
	check := func(what, list string) error {
		set, err := hypervisorhelpers.ParseHostIDList(list, hypervisorhelpers.MaxHostCPUID)
		if err != nil {
			return fmt.Errorf("%s %q: %w", what, list, err)
		}

		if outside := set.Difference(pool); !outside.IsEmpty() {
			return fmt.Errorf("%s names host CPUs %q outside the CPUs %q of its pool", what, outside, pool)
		}

		return nil
	}

	var errs error

	for _, pin := range cpu.Pins {
		errs = errors.Join(errs, check(fmt.Sprintf("vCPU %d pin", pin.VCPU), pin.CPUs))
	}

	if cpu.EmulatorPin != "" {
		errs = errors.Join(errs, check("emulator pin", cpu.EmulatorPin))
	}

	return errs
}
