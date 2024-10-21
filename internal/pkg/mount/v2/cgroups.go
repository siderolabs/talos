// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"path/filepath"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ForceGGroupsV1 returns the cgroup version to be used (only for !container mode).
func ForceGGroupsV1() bool {
	return pointer.SafeDeref(procfs.ProcCmdline().Get(constants.KernelParamCGroups).First()) == "0"
}

// CGroupMountPoints returns the cgroup mount points.
func CGroupMountPoints() Points {
	if ForceGGroupsV1() {
		return cgroupMountPointsV1()
	}

	return cgroupMountPointsV2()
}

func cgroupMountPointsV2() Points {
	return Points{
		NewPoint("cgroup", constants.CgroupMountPath, "cgroup2", WithFlags(unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME), WithData("nsdelegate,memory_recursiveprot")),
	}
}

func cgroupMountPointsV1() Points {
	points := Points{
		NewPoint("tmpfs", constants.CgroupMountPath, "tmpfs", WithFlags(unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME), WithData("mode=755")),
	}

	for _, controller := range []string{
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
	} {
		points = append(points,
			NewPoint("cgroup", filepath.Join(constants.CgroupMountPath, controller), "cgroup", WithFlags(unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME), WithData(controller)),
		)
	}

	return points
}
