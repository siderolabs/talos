// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/hashicorp/go-multierror"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
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
