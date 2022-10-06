// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// OverlayMountPoints returns the mountpoints required to boot the system.
// These mountpoints are used as overlays on top of the read only rootfs.
func OverlayMountPoints() (mountpoints *Points, err error) {
	mountpoints = NewMountPoints()

	for _, target := range constants.Overlays {
		mountpoint := NewMountPoint("", target, "", unix.MS_I_VERSION, "", WithFlags(Overlay))
		mountpoints.Set(target, mountpoint)
	}

	return mountpoints, nil
}
