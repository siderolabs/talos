/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

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
func Mount(s string) error {
	target := path.Join(s, "/sys/fs/cgroup")
	if err := os.MkdirAll(target, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create %s: %s", target, err.Error())
	}
	if err := unix.Mount("tmpfs", target, "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", target, err.Error())
	}

	cgroups := []string{
		"hugetlb",
		"memory",
		"net_cls",
		"perf_event",
		"cpu",
		"devices",
		"pids",
		"blkio",
		"freezer",
		"cpuset",
	}
	for _, c := range cgroups {
		p := path.Join(s, "/sys/fs/cgroup", c)
		if err := os.MkdirAll(p, os.ModeDir); err != nil {
			return fmt.Errorf("failed to create %s: %s", p, err.Error())
		}
		if err := unix.Mount("cgroup", p, "cgroup", 0, ""); err != nil {
			return fmt.Errorf("failed to mount %s: %s", p, err.Error())
		}
	}

	// See https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
	target = path.Join(s, "/sys/fs/cgroup", memoryCgroup, memoryUseHierarchy)
	err := ioutil.WriteFile(target, memoryUseHierarchyContents, memoryUseHierarchyPermissions)

	return err
}
