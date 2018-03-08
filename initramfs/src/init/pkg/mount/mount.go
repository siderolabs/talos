package mount

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"golang.org/x/sys/unix"
)

func enableCgroupMemoryHierarchy() error {
	if err := ioutil.WriteFile("/sys/fs/cgroup/memory.use_hierarchy", []byte{1}, 0644); err != nil {
		return fmt.Errorf("failed to set memory.use_hierarchy: %s", err.Error())
	}

	return nil
}

/*
proc        /proc                        proc     nosuid,noexec,nodev    0   0
sysfs       /sys                         sysfs    nosuid,noexec,nodev    0   0
tmpfs       /run                         tmpfs    defaults               0   0
devtmpfs    /dev                         devtmpfs nosuid                 0   0
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
func Init() error {
	if err := os.MkdirAll("/dev", os.ModeDir); err != nil {
		return fmt.Errorf("failed to create /dev: %s", err.Error())
	}
	if err := unix.Mount("none", "/dev", "devtmpfs", unix.MS_NOSUID, ""); err != nil {
		return fmt.Errorf("failed to mount /dev: %s", err.Error())
	}

	if err := os.MkdirAll("/proc", os.ModeDir); err != nil {
		return fmt.Errorf("failed to create /dev: %s", err.Error())
	}
	if err := unix.Mount("none", "/proc", "proc", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("failed to mount /proc: %s", err.Error())
	}

	if err := os.MkdirAll("/sys", os.ModeDir); err != nil {
		return fmt.Errorf("failed to create /dev: %s", err.Error())
	}
	if err := unix.Mount("none", "/sys", "sysfs", unix.MS_NOSUID|unix.MS_NOEXEC|unix.MS_NODEV, ""); err != nil {
		return fmt.Errorf("failed to mount /sys: %s", err.Error())
	}
	// enableCgroupMemoryHierarchy()
	if err := os.MkdirAll("/run", os.ModeDir); err != nil {
		return fmt.Errorf("failed to create /dev: %s", err.Error())
	}
	if err := unix.Mount("none", "/run", "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount /run: %s", err.Error())
	}

	if err := os.MkdirAll(constants.NewRoot, os.ModeDir); err != nil {
		return fmt.Errorf("failed to create /mnt/newroot: %s", err.Error())
	}

	if err := unix.Mount("/dev/sda1", constants.NewRoot, "xfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", constants.NewRoot, err.Error())
	}
	// arguments, err := parseProcCmdline()
	// if err != nil {
	// 	return err
	// }

	// if root, ok := arguments["dianemo.autonomy.io/root"]; ok {
	// 	return fmt.Errorf("using %s as root disk", root)
	// 	pr, err := blkid.NewProbeFromFilename(root)
	// 	defer blkid.FreeProbe(pr)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to create probe from %s: %s", root, err.Error())
	// 	}

	// 	ls := blkid.ProbeGetPartitions(pr)
	// 	nparts := blkid.ProbeGetPartitionsPartlistNumOfPartitions(ls)

	// 	for i := 0; i < nparts; i++ {
	// 		dev := fmt.Sprintf("%s%d", root, i+1)
	// 		pr, err = blkid.NewProbeFromFilename(dev)
	// 		defer blkid.FreeProbe(pr)
	// 		if err != nil {
	// 			return fmt.Errorf("failed to create probe from %s: %s", dev, err.Error())
	// 		}

	// 		blkid.DoProbe(pr)
	// 		label, _ := blkid.ProbeLookupValue(pr, "LABEL", nil)

	// 		if label == "ROOT" {
	// 			if err := os.MkdirAll(constants.NewRoot, os.ModeDir); err != nil {
	// 				return fmt.Errorf("failed to create /mnt/newroot: %s", err.Error())
	// 			}
	// 			if err := unix.Mount(dev, constants.NewRoot, "xfs", 0, ""); err != nil {
	// 				return fmt.Errorf("failed to mount /mnt/newroot: %s", err.Error())
	// 			}
	// 		}
	// 	}
	// } else {
	// 	return fmt.Errorf("no root specified")
	// }

	return nil
}

func Move() error {
	if err := unix.Mount("/dev", path.Join(constants.NewRoot, "/dev"), "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", path.Join(constants.NewRoot, "/dev"), err.Error())
	}
	if err := unix.Mount("/proc", path.Join(constants.NewRoot, "/proc"), "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", path.Join(constants.NewRoot, "/proc"), err.Error())
	}
	if err := unix.Mount("/sys", path.Join(constants.NewRoot, "/sys"), "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", path.Join(constants.NewRoot, "/sys"), err.Error())
	}
	if err := os.MkdirAll(path.Join(constants.NewRoot, "/sys/fs/cgroup"), os.ModeDir); err != nil {
		return fmt.Errorf("failed to create %s: %s", path.Join(constants.NewRoot, "/sys/fs/cgroup"), err.Error())
	}
	if err := unix.Mount("defaults", path.Join(constants.NewRoot, "/sys/fs/cgroup"), "tmpfs", 0, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", path.Join(constants.NewRoot, "/sys/fs/cgroup"), err.Error())
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
	if err := unix.Mount("/run", path.Join(constants.NewRoot, "/run"), "", unix.MS_MOVE, ""); err != nil {
		return fmt.Errorf("failed to mount %s: %s", path.Join(constants.NewRoot, "/run"), err.Error())
	}

	return nil
}
