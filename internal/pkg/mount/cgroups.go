// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"path"

	"golang.org/x/sys/unix"
)

// CGroupMountPoints returns the cgroup mount points.
func CGroupMountPoints() (mountpoints *Points, err error) {
	base := "/sys/fs/cgroup"
	cgroups := NewMountPoints()
	cgroups.Set("dev", NewMountPoint("tmpfs", base, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, "mode=755"))

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
	for _, c := range controllers {
		p := path.Join(base, c)
		cgroups.Set(c, NewMountPoint(c, p, "cgroup", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, c))
	}

	return cgroups, nil
}
