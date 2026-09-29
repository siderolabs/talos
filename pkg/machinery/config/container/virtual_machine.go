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

// validateVirtualMachineImageReferences checks the content libraries virtual machine disks are
// provisioned from.
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
