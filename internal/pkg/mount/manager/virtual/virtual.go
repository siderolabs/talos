// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package virtual

import (
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/mount"
)

// MountPoints returns the mountpoints required to boot the system.
func MountPoints() (mountpoints *mount.Points, err error) {
	virtual := mount.NewMountPoints()
	virtual.Set("dev", mount.NewMountPoint("devtmpfs", "/dev", "devtmpfs", unix.MS_NOSUID, "mode=0755"))
	virtual.Set("proc", mount.NewMountPoint("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))
	virtual.Set("sys", mount.NewMountPoint("sysfs", "/sys", "sysfs", 0, ""))
	virtual.Set("run", mount.NewMountPoint("tmpfs", "/run", "tmpfs", 0, ""))
	virtual.Set("tmp", mount.NewMountPoint("tmpfs", "/tmp", "tmpfs", 0, ""))

	return virtual, nil
}

// SubMountPoints returns the mountpoints required to boot the system.
func SubMountPoints() (mountpoints *mount.Points, err error) {
	virtual := mount.NewMountPoints()
	virtual.Set("devshm", mount.NewMountPoint("tmpfs", "/dev/shm", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""))
	virtual.Set("devpts", mount.NewMountPoint("devpts", "/dev/pts", "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"))
	virtual.Set("securityfs", mount.NewMountPoint("securityfs", "/sys/kernel/security", "securityfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""))

	return virtual, nil
}
