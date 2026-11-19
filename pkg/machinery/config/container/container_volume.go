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

// validateContainerVolumeMounts checks the volumes mounted into containers.
//
// A volume mount names a UserVolumeConfig, ExistingVolumeConfig, ExternalVolumeConfig or
// InlineDataVolumeConfig, and is resolved against the rest of the configuration, so a name nothing
// declares would leave the container waiting for its volume forever. A mount request is read-only
// only while every requester asks for it, so a writable mount of a volume declared read-only would
// remount the volume against what the user declared; such a mount has to say `ro`. An inline data
// volume is always read-only, so `ro` is implied for it.
func validateContainerVolumeMounts(container *Container) error {
	configs := container.ContainerConfigs()

	if len(configs) == 0 {
		return nil
	}

	// Sorted by name, so a given configuration always reports errors in the same order.
	sorted := slices.SortedFunc(slices.Values(configs), func(a, b config.ContainerConfig) int {
		return cmp.Compare(a.Name(), b.Name())
	})

	var errs *multierror.Error

	for _, containerConfig := range sorted {
		for i, mount := range containerConfig.Mounts() {
			volumeMount, ok := mount.Volume().Get()
			if !ok || volumeMount.Name() == "" {
				// Not a volume mount, or a missing name the document's own validation reports.
				continue
			}

			backing, resolved := config.ResolveBackingVolume(container, volumeMount.Name())

			switch {
			case !resolved:
				errs = multierror.Append(errs, fmt.Errorf(
					"container %q: mounts[%d]: no UserVolumeConfig, ExistingVolumeConfig, ExternalVolumeConfig or InlineDataVolumeConfig declares volume %q",
					containerConfig.Name(), i, volumeMount.Name(),
				))
			case backing.InlineData:
				errs = multierror.Append(errs, validateInlineDataVolumeMount(containerConfig.Name(), i, volumeMount))
			case backing.ReadOnly && !slices.Contains(volumeMount.MountOptions(), "ro"):
				errs = multierror.Append(errs, fmt.Errorf(
					"container %q: mounts[%d]: volume %q is configured read-only, so it must be mounted with the ro option",
					containerConfig.Name(), i, volumeMount.Name(),
				))
			}
		}
	}

	return errs.ErrorOrNil()
}

// validateInlineDataVolumeMount rejects the options an inline data volume mount can't honor.
//
// The mount is always ro and rprivate, so `ro` may be left out, but asking for `rw` or `rshared`
// would be silently ignored.
func validateInlineDataVolumeMount(containerName string, i int, volumeMount config.ContainerVolumeMountConfig) error {
	var errs *multierror.Error

	if volumeMount.Writable() {
		errs = multierror.Append(errs, fmt.Errorf(
			"container %q: mounts[%d]: inline data volume %q is always read-only, so it can't be mounted with the rw option",
			containerName, i, volumeMount.Name(),
		))
	}

	if slices.Contains(volumeMount.MountOptions(), "rshared") {
		errs = multierror.Append(errs, fmt.Errorf(
			"container %q: mounts[%d]: inline data volume %q is always mounted rprivate, so it can't be mounted with the rshared option",
			containerName, i, volumeMount.Name(),
		))
	}

	return errs.ErrorOrNil()
}
