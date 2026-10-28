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

// validateContainerRawVolumeDevices checks that raw volume devices name a declared RawVolumeConfig,
// and that a raw volume is handed to a single container.
func validateContainerRawVolumeDevices(container *Container) error {
	configs := container.ContainerConfigs()

	if len(configs) == 0 {
		return nil
	}

	rawVolumes := map[string]struct{}{}

	for _, rawVolumeConfig := range container.RawVolumeConfigs() {
		rawVolumes[rawVolumeConfig.Name()] = struct{}{}
	}

	// Sorted by name, so a given configuration always reports errors in the same order.
	sorted := slices.SortedFunc(slices.Values(configs), func(a, b config.ContainerConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	var errs *multierror.Error

	owners := map[string]string{}

	for _, containerConfig := range sorted {
		for i, device := range containerConfig.Devices() {
			rawVolumeDevice, ok := device.RawVolume().Get()
			if !ok || rawVolumeDevice.Name() == "" {
				// Not a raw volume device, or a missing name the document's own validation reports.
				continue
			}

			name := rawVolumeDevice.Name()

			if _, declared := rawVolumes[name]; !declared {
				errs = multierror.Append(errs, fmt.Errorf(
					"container %q: devices[%d]: no RawVolumeConfig declares raw volume %q",
					containerConfig.Name(), i, name,
				))

				continue
			}

			owner, owned := owners[name]

			switch {
			case !owned:
				owners[name] = containerConfig.Name()
			case owner != containerConfig.Name():
				errs = multierror.Append(errs, fmt.Errorf(
					"container %q: devices[%d]: raw volume %q is already handed to container %q",
					containerConfig.Name(), i, name, owner,
				))
			}
		}
	}

	return errs.ErrorOrNil()
}
