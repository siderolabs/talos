// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"golang.org/x/sys/unix"
	"gopkg.in/freddierice/go-losetup.v1"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// SquashfsMountPoints returns the mountpoints required to boot the system.
func SquashfsMountPoints(prefix string) (mountpoints *Points, err error) {
	var dev losetup.Device

	dev, err = losetup.Attach("/"+constants.RootfsAsset, 0, true)
	if err != nil {
		return nil, err
	}

	squashfs := NewMountPoints()
	squashfs.Set("squashfs", NewMountPoint(dev.Path(), "/", "squashfs", unix.MS_RDONLY|unix.MS_I_VERSION, "", WithPrefix(prefix), WithFlags(ReadOnly|Shared)))

	return squashfs, nil
}
