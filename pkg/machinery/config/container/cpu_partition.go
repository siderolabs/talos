// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// kubeletReservedSystemCPUsField is the kubelet configuration field Talos owns while kubepods is bounded.
const kubeletReservedSystemCPUsField = "reservedSystemCPUs"

// validateCPUPartition checks what a CPUPartitionConfig means for the rest of the configuration:
// virtual machines referencing slices, exclusive slice ownership, host CPU pins against the
// allocation, and the kubelet reservation Talos takes over.
//
// Each document has already been validated on its own, so unparseable CPU lists are skipped
// here rather than reported twice.
//
//nolint:gocyclo,cyclop
func validateCPUPartition(container *Container) error {
	partition := container.CPUPartitionConfig()

	// Sorted by name, so the same configuration always reports its problems in the same order.
	virtualMachines := slices.SortedFunc(slices.Values(container.VirtualMachineConfigs()), func(a, b config.VirtualMachineConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	var errs *multierror.Error

	if partition == nil {
		for _, vm := range virtualMachines {
			if vm.CPU().Slice() != "" {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: cpu.slice %q: no CPUPartitionConfig declares any slice",
					vm.Name(), vm.CPU().Slice()))
			}
		}

		return errs.ErrorOrNil()
	}

	roots := partition.Roots()

	if _, bounded := roots[config.CPUPartitionRootKubepods]; bounded {
		if kubelet := container.K8sKubeletConfig(); kubelet != nil {
			if _, set := kubelet.ExtraConfig()[kubeletReservedSystemCPUsField]; set {
				errs = multierror.Append(errs, fmt.Errorf(
					"kubelet configuration field %q is owned by CPUPartitionConfig while kubepods is bounded",
					kubeletReservedSystemCPUsField))
			}
		}
	}

	vmRoot, vmRootBounded := parseValidatedCPUList(roots[config.CPUPartitionRootVirtualMachines])

	type sliceAllocation struct {
		cpus      cpuset.CPUSet
		exclusive bool
	}

	allocations := map[string]sliceAllocation{}
	remainder := vmRoot

	for _, slice := range partition.Slices() {
		cpus, ok := parseValidatedCPUList(slice.CPUs())
		if !ok {
			continue
		}

		allocations[slice.Name()] = sliceAllocation{cpus: cpus, exclusive: slice.Exclusive()}
		remainder = remainder.Difference(cpus)
	}

	exclusiveOwners := map[string][]string{}

	for _, vm := range virtualMachines {
		name := vm.Name()
		sliceName := vm.CPU().Slice()

		var allocation cpuset.CPUSet

		switch sliceName {
		case "":
			if !vmRootBounded {
				// No virtual machine root: the machine runs on any host CPU, pins are unconstrained.
				continue
			}

			if remainder.IsEmpty() {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: no cpu.slice selected but the named slices leave no CPU of virtualMachines.cpus %q",
					name, vmRoot))

				continue
			}

			allocation = remainder
		default:
			slice, declared := allocations[sliceName]
			if !declared {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: cpu.slice %q: no CPUPartitionConfig declares slice %q", name, sliceName, sliceName))

				continue
			}

			if slice.exclusive {
				exclusiveOwners[sliceName] = append(exclusiveOwners[sliceName], name)
			}

			allocation = slice.cpus
		}

		pinning := vm.CPU().Topology().Pinning()

		for i, pin := range pinning.VCPUs() {
			if cpus, ok := parseValidatedCPUList(pin.CPUs()); ok && !cpus.IsSubsetOf(allocation) {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: cpu.topology.pinning.vcpus[%d].cpus %q must be a subset of %s",
					name, i, cpus, describeAllocation(sliceName, allocation)))
			}
		}

		if cpus, ok := parseValidatedCPUList(pinning.Emulator()); ok && !cpus.IsEmpty() && !cpus.IsSubsetOf(allocation) {
			errs = multierror.Append(errs, fmt.Errorf(
				"virtual machine %q: cpu.topology.pinning.emulator %q must be a subset of %s",
				name, cpus, describeAllocation(sliceName, allocation)))
		}
	}

	for _, sliceName := range slices.Sorted(func(yield func(string) bool) {
		for sliceName := range exclusiveOwners {
			if !yield(sliceName) {
				return
			}
		}
	}) {
		if owners := exclusiveOwners[sliceName]; len(owners) > 1 {
			errs = multierror.Append(errs, fmt.Errorf(
				"exclusive slice %q is selected by more than one virtual machine: %v", sliceName, owners))
		}
	}

	return errs.ErrorOrNil()
}

// parseValidatedCPUList parses a CPU list that already passed document validation; an empty or
// unparseable list yields false so it is not reported twice.
func parseValidatedCPUList(list string) (cpuset.CPUSet, bool) {
	if list == "" {
		return cpuset.New(), false
	}

	set, err := hypervisorhelpers.ParseHostIDList(list, hypervisorhelpers.MaxHostCPUID)
	if err != nil {
		return cpuset.New(), false
	}

	return set, true
}

func describeAllocation(sliceName string, allocation cpuset.CPUSet) string {
	if sliceName == "" {
		return fmt.Sprintf("the shared remainder %q of virtualMachines.cpus", allocation)
	}

	return fmt.Sprintf("slice %q (%q)", sliceName, allocation)
}
