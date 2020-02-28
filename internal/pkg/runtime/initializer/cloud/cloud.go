// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cloud

import (
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Cloud is an initializer that mounts an existing installation.
type Cloud struct{}

// Initialize implements the Initializer interface.
func (c *Cloud) Initialize(r runtime.Runtime) (err error) {
	mountpoints := mount.NewMountPoints()

	mountpoint, err := owned.MountPointForLabel(constants.EphemeralPartitionLabel)
	if err != nil {
		return err
	}

	mountpoints.Set(constants.EphemeralPartitionLabel, mountpoint)

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
