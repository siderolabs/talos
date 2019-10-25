// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package overlay

import (
	"github.com/talos-systems/talos/internal/pkg/mount"
)

// MountPoints returns the mountpoints required to boot the system.
// These moiuntpoints are used as overlays on top of the read only rootfs.
func MountPoints() (mountpoints *mount.Points, err error) {
	mountpoints = mount.NewMountPoints()

	overlays := []string{
		"/etc/kubernetes",
		"/etc/cni",
		"/usr/libexec/kubernetes",
		"/usr/etc/udev",
		"/opt",
	}

	for _, target := range overlays {
		mountpoint := mount.NewMountPoint("", target, "", 0, "", mount.WithOverlay(true))
		mountpoints.Set(target, mountpoint)
	}

	return mountpoints, nil
}
