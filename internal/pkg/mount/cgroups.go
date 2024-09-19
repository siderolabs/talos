// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"path/filepath"

	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ForceGGroupsV1 returns the cgroup version to be used (only for !container mode).
func ForceGGroupsV1() bool {
	value := procfs.ProcCmdline().Get(constants.KernelParamCGroups).First()

	return value != nil && *value == "0"
}

// CGroupMountPoints returns the cgroup mount points.
func CGroupMountPoints() (mountpoints *Points, err error) {
	if ForceGGroupsV1() {
		return cgroupMountPointsV1()
	}

	return cgroupMountPointsV2()
}

func cgroupMountPointsV2() (mountpoints *Points, err error) {
	cgroups := NewMountPoints()

	cgroups.Set("cgroup2", NewMountPoint("cgroup", constants.CgroupMountPath, "cgroup2", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, "nsdelegate,memory_recursiveprot"))

	return cgroups, nil
}

func cgroupMountPointsV1() (mountpoints *Points, err error) {
	cgroups := NewMountPoints()
	cgroups.Set("dev", NewMountPoint("tmpfs", constants.CgroupMountPath, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, "mode=755"))

	controllers := []string{
		"blkio",
		"cpu",
		"cpuacct",
		"cpuset",
		"devices",
		"freezer",
		"hugetlb",
		"memory",
		"net_cls",
		"net_prio",
		"perf_event",
		"pids",
	}

	for _, controller := range controllers {
		p := filepath.Join(constants.CgroupMountPath, controller)
		cgroups.Set(controller, NewMountPoint(controller, p, "cgroup", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, controller))
	}

	return cgroups, nil
}
