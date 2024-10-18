// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import "golang.org/x/sys/unix"

// Pseudo returns the mountpoints required to boot the system.
func Pseudo() Points {
	return Points{
		NewPoint("devtmpfs", "/dev", "devtmpfs", WithFlags(unix.MS_NOSUID), WithData("mode=0755")),
		NewPoint("proc", "/proc", "proc", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV)),
		NewPoint("sysfs", "/sys", "sysfs"),
		NewPoint("tmpfs", "/run", "tmpfs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_RELATIME), WithData("mode=0755")),
		NewPoint("tmpfs", "/system", "tmpfs", WithData("mode=0755")),
		NewPoint("tmpfs", "/tmp", "tmpfs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV), WithData("size=64M"), WithData("mode=0755")),
	}
}
