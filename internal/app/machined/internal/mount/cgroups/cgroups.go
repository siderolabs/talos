/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cgroups

import (
	"io/ioutil"
	"os"
	"path"
	"strconv"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

const (
	memoryCgroup                  = "memory"
	memoryUseHierarchy            = "memory.use_hierarchy"
	memoryUseHierarchyPermissions = os.FileMode(400)
)

var (
	memoryUseHierarchyContents = []byte(strconv.Itoa(1))
)

// Mount creates the cgroup mount points.
func Mount() error {
	target := "/sys/fs/cgroup"
	if err := os.MkdirAll(target, os.ModeDir); err != nil {
		return errors.Errorf("failed to create %s: %+v", target, err)
	}
	if err := unix.Mount("tmpfs", target, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, "mode=755"); err != nil {
		return errors.Errorf("failed to mount %s: %+v", target, err)
	}

	cgroups := []string{
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
	for _, c := range cgroups {
		p := path.Join("/sys/fs/cgroup", c)
		if err := os.MkdirAll(p, os.ModeDir); err != nil {
			return errors.Errorf("failed to create %s: %+v", p, err)
		}
		if err := unix.Mount(c, p, "cgroup", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC|unix.MS_RELATIME, c); err != nil {
			return errors.Errorf("failed to mount %s: %+v", p, err)
		}
	}

	// See https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
	target = path.Join("/sys/fs/cgroup", memoryCgroup, memoryUseHierarchy)
	err := ioutil.WriteFile(target, memoryUseHierarchyContents, memoryUseHierarchyPermissions)

	return err
}
