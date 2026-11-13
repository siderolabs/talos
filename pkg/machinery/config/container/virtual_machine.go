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
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// validateVirtualMachinePoolReferences checks pools for virtual machine disks and cloud-init seeds.
func validateVirtualMachinePoolReferences(container *Container) error {
	configs := container.VirtualMachineConfigs()

	if len(configs) == 0 {
		return nil
	}

	// Sorted by name, so the same configuration always reports its problems in the same order.
	sorted := slices.SortedFunc(slices.Values(configs), func(a, b config.VirtualMachineConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	declaredPools := map[string]struct{}{}

	for _, storagePoolConfig := range container.StoragePoolConfigs() {
		declaredPools[storagePoolConfig.Name()] = struct{}{}
	}

	var errs *multierror.Error

	for _, virtualMachineConfig := range sorted {
		if cloudInit, ok := virtualMachineConfig.Guest().CloudInit().Get(); ok && cloudInit.Pool() != "" {
			if _, declared := declaredPools[cloudInit.Pool()]; !declared {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: guest.cloudInit: no StoragePoolConfig document declares storage pool %q",
					virtualMachineConfig.Name(), cloudInit.Pool()))
			}
		}

		for i, disk := range virtualMachineConfig.Disks() {
			poolName := disk.Pool()

			if poolName == "" {
				// A missing pool name is reported by the document's own validation.
				continue
			}

			if _, declared := declaredPools[poolName]; !declared {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: disks[%d]: no StoragePoolConfig document declares storage pool %q",
					virtualMachineConfig.Name(), i, poolName))
			}
		}
	}

	return errs.ErrorOrNil()
}

// validateVirtualMachineCPUSlices checks cpu.slice against the CPU partition policy as written; the
// running node keeps the slices of its boot policy until a reboot. A virtual machine without a slice
// needs the CPUs of the root outside every named slice.
func validateVirtualMachineCPUSlices(container *Container) error {
	declared, shared, bounded := virtualMachineCPUSlices(container.CPUPartitionConfig())
	if !bounded {
		return nil
	}

	var errs *multierror.Error

	for _, virtualMachineConfig := range container.VirtualMachineConfigs() {
		slice := virtualMachineConfig.CPU().Slice()

		switch _, ok := declared[slice]; {
		case slice != "" && !ok:
			errs = multierror.Append(errs, fmt.Errorf(
				"virtual machine %q: cpu.slice: CPUPartitionConfig declares no slice %q", virtualMachineConfig.Name(), slice))
		case slice == "" && shared.IsEmpty():
			errs = multierror.Append(errs, fmt.Errorf(
				"virtual machine %q: cpu.slice: the CPUPartitionConfig slices take every CPU of the virtual machine root, select one of them",
				virtualMachineConfig.Name()))
		}
	}

	return errs.ErrorOrNil()
}

// virtualMachineCPUSlices returns the declared slice names and the CPUs of the virtual machine root
// outside them, and whether the policy bounds a valid virtual machine root at all. Invalid lists are
// reported by the document's own validation.
func virtualMachineCPUSlices(partition config.CPUPartitionConfig) (map[string]struct{}, cpuset.CPUSet, bool) {
	if partition == nil {
		return nil, cpuset.New(), false
	}

	shared, err := cpuset.Parse(partition.Roots()[constants.CgroupVirtualMachinesRoot])
	if err != nil || shared.IsEmpty() {
		return nil, cpuset.New(), false
	}

	declared := map[string]struct{}{}

	for _, slice := range partition.Slices() {
		declared[slice.Name()] = struct{}{}

		if cpus, err := cpuset.Parse(slice.CPUs()); err == nil {
			shared = shared.Difference(cpus)
		}
	}

	return declared, shared, true
}
