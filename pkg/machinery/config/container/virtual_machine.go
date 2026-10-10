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

// validateVirtualMachineImageReferences checks the content libraries used by virtual machine disks
// and cloud-init seed assets.
func validateVirtualMachineImageReferences(container *Container) error {
	configs := container.VirtualMachineConfigs()

	if len(configs) == 0 {
		return nil
	}

	// Sorted by name, so the same configuration always reports its problems in the same order.
	sorted := slices.SortedFunc(slices.Values(configs), func(a, b config.VirtualMachineConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	declaredLibraries := map[string]struct{}{}

	for _, contentLibraryConfig := range container.ContentLibraryConfigs() {
		declaredLibraries[contentLibraryConfig.Name()] = struct{}{}
	}

	var errs *multierror.Error

	for _, virtualMachineConfig := range sorted {
		if err := validateVirtualMachineCloudInitLibrary(virtualMachineConfig, declaredLibraries); err != nil {
			errs = multierror.Append(errs, err)
		}

		for i, disk := range virtualMachineConfig.Disks() {
			fromImage, ok := disk.Provision().FromImage().Get()
			if !ok {
				continue
			}

			libraryName := fromImage.Library()

			if libraryName == "" {
				// A missing library name is reported by the document's own validation.
				continue
			}

			if _, declared := declaredLibraries[libraryName]; !declared {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: disks[%d]: no ContentLibraryConfig declares content library %q",
					virtualMachineConfig.Name(), i, libraryName))
			}
		}
	}

	return errs.ErrorOrNil()
}

func validateVirtualMachineCloudInitLibrary(vm config.VirtualMachineConfig, declaredLibraries map[string]struct{}) error {
	cloudInit, ok := vm.Guest().CloudInit().Get()
	if !ok {
		return nil
	}

	libraryName := cloudInit.Library()
	if libraryName == "" {
		// A missing library name is reported by the document's own validation.
		return nil
	}

	if _, declared := declaredLibraries[libraryName]; !declared {
		return fmt.Errorf("virtual machine %q: guest.cloudInit: no ContentLibraryConfig declares content library %q", vm.Name(), libraryName)
	}

	return nil
}

// validateVirtualMachinePoolReferences checks the storage pools virtual machine disks live in.
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
		for i, disk := range virtualMachineConfig.Disks() {
			poolName := disk.Pool()

			if poolName == "" {
				// A missing pool name, where one is required, is reported by the document's own
				// validation; a cdrom is allowed none at all.
				continue
			}

			if _, declared := declaredPools[poolName]; !declared {
				errs = multierror.Append(errs, fmt.Errorf(
					"virtual machine %q: disks[%d]: no StoragePool document declares storage pool %q",
					virtualMachineConfig.Name(), i, poolName))
			}
		}
	}

	return errs.ErrorOrNil()
}
