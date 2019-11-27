// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pseudo

import (
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
)

// MountPoints returns the mountpoints required to boot the system.
func MountPoints() (mountpoints *mount.Points, err error) {
	pseudo := mount.NewMountPoints()
	pseudo.Set("dev", mount.NewMountPoint("devtmpfs", "/dev", "devtmpfs", unix.MS_NOSUID, "mode=0755"))
	pseudo.Set("proc", mount.NewMountPoint("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))
	pseudo.Set("sys", mount.NewMountPoint("sysfs", "/sys", "sysfs", 0, ""))
	pseudo.Set("run", mount.NewMountPoint("tmpfs", "/run", "tmpfs", 0, ""))
	pseudo.Set("tmp", mount.NewMountPoint("tmpfs", "/tmp", "tmpfs", 0, ""))

	return pseudo, nil
}

// SubMountPoints returns the mountpoints required to boot the system.
func SubMountPoints() (mountpoints *mount.Points, err error) {
	pseudo := mount.NewMountPoints()
	pseudo.Set("devshm", mount.NewMountPoint("tmpfs", "/dev/shm", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""))
	pseudo.Set("devpts", mount.NewMountPoint("devpts", "/dev/pts", "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"))
	pseudo.Set("securityfs", mount.NewMountPoint("securityfs", "/sys/kernel/security", "securityfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""))

	return pseudo, nil
}
