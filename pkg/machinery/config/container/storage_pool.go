// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"errors"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// validateStoragePools checks references against the complete configuration, not just one document.
func (container *Container) validateStoragePools() error {
	type backingVolume struct {
		readOnly bool
	}

	volumes := map[string]backingVolume{}

	for _, volume := range container.UserVolumeConfigs() {
		// UserVolumeConfig validation already restricts these to filesystem-backed
		// partitions, disks, or directories. Filesystem "none" defaults to XFS.
		volumes[constants.UserVolumePrefix+volume.Name()] = backingVolume{}
	}

	for _, volume := range container.ExistingVolumeConfigs() {
		// The filesystem is discovered at runtime; the mount controller must verify it.
		volumes[constants.ExistingVolumePrefix+volume.Name()] = backingVolume{readOnly: volume.Mount().ReadOnly()}
	}

	for _, volume := range container.ExternalVolumeConfigs() {
		volumes[constants.ExternalVolumePrefix+volume.Name()] = backingVolume{readOnly: volume.Mount().ReadOnly()}
	}

	var errs error

	owners := map[string]string{}

	for _, pool := range container.StoragePoolConfigs() {
		name := pool.VolumeName()
		volume, exists := volumes[name]

		if !exists {
			errs = errors.Join(errs, fmt.Errorf("storage pool %q: volume.name %q must reference a UserVolumeConfig, ExistingVolumeConfig, or ExternalVolumeConfig", pool.Name(), name))
		} else if volume.readOnly {
			errs = errors.Join(errs, fmt.Errorf("storage pool %q: backing volume %q is read-only", pool.Name(), name))
		}

		if owner, exists := owners[name]; exists {
			errs = errors.Join(errs, fmt.Errorf("storage pools %q and %q reference the same backing volume %q", owner, pool.Name(), name))
		} else {
			owners[name] = pool.Name()
		}
	}

	return errs
}
