// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"os"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// PseudoMountPoints returns the mountpoints required to boot the system.
func PseudoMountPoints() (mountpoints *Points, err error) {
	pseudo := NewMountPoints()
	pseudo.Set("dev", NewMountPoint("devtmpfs", "/dev", "devtmpfs", unix.MS_NOSUID, "mode=0755"))
	pseudo.Set("proc", NewMountPoint("proc", "/proc", "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))
	pseudo.Set("sys", NewMountPoint("sysfs", "/sys", "sysfs", 0, ""))
	pseudo.Set("run", NewMountPoint("tmpfs", "/run", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_RELATIME, "mode=755"))
	pseudo.Set("system", NewMountPoint("tmpfs", "/system", "tmpfs", 0, "mode=755"))
	pseudo.Set("tmp", NewMountPoint("tmpfs", "/tmp", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, "size=64M,mode=755"))

	return pseudo, nil
}

// PseudoSubMountPoints returns the mountpoints required to boot the system.
func PseudoSubMountPoints() (mountpoints *Points, err error) {
	pseudo := NewMountPoints()
	pseudo.Set("devshm", NewMountPoint("tmpfs", "/dev/shm", "tmpfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""))
	pseudo.Set("devpts", NewMountPoint("devpts", "/dev/pts", "devpts", unix.MS_NOSUID|unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"))
	pseudo.Set("hugetlb", NewMountPoint("hugetlbfs", "/dev/hugepages", "hugetlbfs", unix.MS_NOSUID|unix.MS_NODEV, ""))
	pseudo.Set("securityfs", NewMountPoint("securityfs", "/sys/kernel/security", "securityfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME, ""))
	pseudo.Set("tracefs", NewMountPoint("securityfs", "/sys/kernel/tracing", "tracefs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""))

	if _, err := os.Stat(constants.EFIVarsMountPoint); err == nil {
		// mount EFI vars if they exist
		pseudo.Set("efivars", NewMountPoint("efivarfs", constants.EFIVarsMountPoint, "efivarfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV|unix.MS_RELATIME|unix.MS_RDONLY, "",
			WithFlags(SkipIfNoDevice),
		))
	}

	return pseudo, nil
}
