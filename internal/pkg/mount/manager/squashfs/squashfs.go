/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package squashfs

import (
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"golang.org/x/sys/unix"
	"gopkg.in/freddierice/go-losetup.v1"
)

// MountPoints returns the mountpoints required to boot the system.
func MountPoints(prefix string) (mountpoints *mount.Points, err error) {
	var dev losetup.Device
	dev, err = losetup.Attach("/"+constants.RootfsAsset, 0, true)
	if err != nil {
		return nil, err
	}
	squashfs := mount.NewMountPoints()
	squashfs.Set("squashfs", mount.NewMountPoint(dev.Path(), "/", "squashfs", unix.MS_RDONLY, "", mount.WithPrefix(prefix), mount.WithReadOnly(true), mount.WithShared(true)))

	return squashfs, nil
}
