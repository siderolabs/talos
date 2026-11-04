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

// Select returns the partition of the selected slice, or of the shared pool, and checks the host
// CPU pins against that pool. Without pools the virtual machine root is not partitioned, so neither applies.
func Select(name string, pools []runtime.CPUPartitionPoolSpec, cpu hypervisor.VirtualMachineCPUSpec) (string, error) {
	unavailable := func(err error) error {
		return fmt.Errorf("virtual machine %q: %w: %w", name, ErrUnavailable, err)
	}

	if len(pools) == 0 {
		return "/" + constants.CgroupVirtualMachines, nil
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
			return "", unavailable(fmt.Errorf("invalid CPUs %q of pool %q: %w", pool.CPUs, pool.Name, err))
		}

		if err = checkPins(cpu, cpus); err != nil {
			return "", unavailable(err)
		}

		return pool.Partition, nil
	}

	if cpu.Slice == "" {
		return "", unavailable(errors.New("the named slices take every CPU of the virtual machine root, select one of them"))
	}

	return "", unavailable(fmt.Errorf("CPU slice %q is not declared by the CPU partition policy of the boot", cpu.Slice))
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
