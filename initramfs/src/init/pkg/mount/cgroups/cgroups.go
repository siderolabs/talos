package cgroups

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"golang.org/x/sys/unix"
)

func enableMemoryHierarchy() error {
	f := path.Join(constants.NewRoot, "/sys/fs/cgroup/memory.use_hierarchy")
	if err := ioutil.WriteFile(f, []byte{1}, 0644); err != nil {
		return fmt.Errorf("set memory.use_hierarchy: %s", err.Error())
	}

	return nil
}

/*
Mount creates the following file systems:
	cgroup      /sys/fs/cgroup               tmpfs    defaults               0   0
	cgroup      /sys/fs/cgroup/hugetlb       cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/memory        cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/net_cls       cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/perf_event    cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/cpu           cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/devices       cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/pids          cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/blkio         cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/freezer       cgroup   defaults               0   0
	cgroup      /sys/fs/cgroup/cpuset        cgroup   defaults               0   0
*/
func Mount() error {
	target := path.Join(constants.NewRoot, "/sys/fs/cgroup")
	if err := os.MkdirAll(target, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create %s: %s", target, err.Error())
	}
	if err := unix.Mount("defaults", target, "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", target, err.Error())
	}

	if err := enableMemoryHierarchy(); err != nil {
		return fmt.Errorf("failed to enable cgroup memory hierarchy: %s", err.Error())
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
		p := path.Join(constants.NewRoot, fmt.Sprintf("/sys/fs/cgroup/%s", c))
		if err := os.MkdirAll(p, os.ModeDir); err != nil {
			return fmt.Errorf("failed to create %s: %s", p, err.Error())
		}
		if err := unix.Mount("defaults", p, "cgroup", 0, ""); err != nil {
			return fmt.Errorf("failed to mount %s: %s", p, err.Error())
		}
	}

	return nil
}
