// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal

import (
	"errors"

	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Metal represents an initializer that performs a full installation to a
// disk.
type Metal struct{}

// Initialize implements the Initializer interface.
func (b *Metal) Initialize(r runtime.Runtime) (err error) {
	// Attempt to discover a previous installation
	// An err case should only happen if no partitions
	// with matching labels were found
	mountpoints := mount.NewMountPoints()

	mountpoint, err := owned.MountPointForLabel(constants.EphemeralPartitionLabel)
	if err != nil {
		if r.Config().Machine().Install().Image() == "" {
			return errors.New("an install image is required")
		}

		if err = install.RunInstallerContainer(r); err != nil {
			return err
		}

		panic(runtime.ErrReboot)
	}

	mountpoints.Set(constants.EphemeralPartitionLabel, mountpoint)

	m := manager.NewManager(mountpoints)
	if err = m.MountAll(); err != nil {
		return err
	}

	return nil
}
