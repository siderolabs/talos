// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"github.com/freddierice/go-losetup/v2"
	"golang.org/x/sys/unix"
)

// Squashfs binds the squashfs to the loop device and returns the mountpoint for it to the specified target.
func Squashfs(target, squashfsFile string) (*Point, error) {
	dev, err := losetup.Attach(squashfsFile, 0, true)
	if err != nil {
		return nil, err
	}

	return NewPoint(dev.Path(), target, "squashfs", WithFlags(unix.MS_RDONLY|unix.MS_I_VERSION), WithShared()), nil
}
