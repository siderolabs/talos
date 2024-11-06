// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"os"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

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

// PseudoSubMountPoints returns the sub-mountpoints under Pseudo().
func PseudoSubMountPoints() Points {
	points := Points{
		NewPoint("devshm", "/dev/shm", "tmpfs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME)),
		NewPoint("devpts", "/dev/pts", "devpts", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC), WithData("ptmxmode=000,mode=620,gid=5")),
		NewPoint("hugetlb", "/dev/hugepages", "hugetlbfs", WithFlags(unix.MS_NOSUID|unix.MS_NODEV)),
		NewPoint("bpf", "/sys/fs/bpf", "bpf"),
		NewPoint("securityfs", "/sys/kernel/security", "securityfs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME)),
		NewPoint("tracefs", "/sys/kernel/tracing", "tracefs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV)),
	}

	if _, err := os.Stat(constants.EFIVarsMountPoint); err == nil {
		// mount EFI vars if they exist
		points = append(points,
			NewPoint("efivars", constants.EFIVarsMountPoint, "efivarfs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME|unix.MS_RDONLY)),
		)
	}

	if _, err := os.Stat("/sys/fs/selinux"); err == nil {
		// mount selinuxfs if it exists
		points = append(points,
			NewPoint("selinuxfs", "/sys/fs/selinux", "selinuxfs", WithFlags(unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_RELATIME)),
		)
	}

	return points
}
