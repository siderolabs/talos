package mount

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/blkid"
	"github.com/autonomy/dianemo/initramfs/src/init/pkg/constants"
	"golang.org/x/sys/unix"
)

type (
	// Filesystem represents a linux file system.
	Filesystem struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}

	// BlockDevice represents the metadata on a block device probed by
	// libblkid.
	BlockDevice struct {
		dev   string
		TYPE  string
		UUID  string
		LABEL string
	}
)

var (
	filesystems = []*Filesystem{
		{
			"none",
			"/dev",
			"devtmpfs",
			unix.MS_NOSUID,
			"",
		},
		{
			"none",
			"/proc",
			"proc",
			unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			"",
		},
		{
			"none",
			"/sys",
			"sysfs",
			unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV,
			"",
		},
		{
			"none",
			"/run",
			"tmpfs",
			0,
			"",
		},
		{
			"none",
			"/tmp",
			"tmpfs",
			0,
			"",
		},
	}
)

/*
Mount creates the following file systems:
	devtmpfs    /dev  devtmpfs nosuid                 0   0
	proc        /proc proc     nosuid,noexec,nodev    0   0
	sysfs       /sys  sysfs    nosuid,noexec,nodev    0   0
	tmpfs       /run  tmpfs    defaults               0   0
	tmpfs       /tmp  tmpfs    defaults               0   0
*/
func Mount() error {
	for _, m := range filesystems {
		if err := os.MkdirAll(m.target, os.ModeDir); err != nil {
			return fmt.Errorf("failed to create %s: %s", m.target, err.Error())
		}
		if err := unix.Mount(m.source, m.target, m.fstype, m.flags, m.data); err != nil {
			return fmt.Errorf("failed to mount %s: %s", m.target, err.Error())
		}
	}

	return nil
}

/*
Move moves the following file systems to the new root:
	devtmpfs    /dev  devtmpfs nosuid                 0   0
	proc        /proc proc     nosuid,noexec,nodev    0   0
	sysfs       /sys  sysfs    nosuid,noexec,nodev    0   0
	tmpfs       /run  tmpfs    defaults               0   0
	tmpfs       /tmp  tmpfs    defaults               0   0
*/
func Move() error {
	if err := os.MkdirAll(constants.NewRoot, os.ModeDir); err != nil {
		return err
	}

	blockDevices, err := probe()
	if err != nil {
		return fmt.Errorf("failed to probe block devices: %s", err.Error())
	}
	for _, b := range blockDevices {
		switch b.LABEL {
		case constants.ROOTLabel:
			if err := unix.Mount(b.dev, constants.NewRoot, b.TYPE, unix.MS_RDONLY|unix.MS_NOATIME, ""); err != nil {
				return fmt.Errorf("mount %s: %s", constants.NewRoot, err.Error())
			}
			// See http://man7.org/linux/man-pages/man2/mount.2.html
			// MS_SHARED
			//   Make this mount point shared.  Mount and unmount events
			//   immediately under this mount point will propagate to the other
			//   mount points that are members of this mount's peer group.
			//   Propagation here means that the same mount or unmount will
			//   automatically occur under all of the other mount points in the
			//   peer group.  Conversely, mount and unmount events that take
			//   place under peer mount points will propagate to this mount
			//   point.
			// https://github.com/kubernetes/kubernetes/issues/61058
			if err := unix.Mount("", constants.NewRoot, "", unix.MS_SHARED, ""); err != nil {
				return fmt.Errorf("mount shared %s: %s", constants.NewRoot, err.Error())
			}
		case constants.DATALabel:
			target := path.Join(constants.NewRoot, "var")
			if err := unix.Mount(b.dev, target, b.TYPE, unix.MS_NOATIME, ""); err != nil {
				return fmt.Errorf("mount %s: %s", target, err.Error())
			}
		}
	}

	// Move the existing file systems to the new root.
	for _, m := range filesystems {
		t := path.Join(constants.NewRoot, m.target)
		if err := unix.Mount(m.target, t, "", unix.MS_MOVE, ""); err != nil {
			return fmt.Errorf("failed to mount %s: %s", t, err.Error())
		}
	}

	return nil
}

func parseProcCmdline() (cmdline map[string]string, err error) {
	cmdline = map[string]string{}
	cmdlineBytes, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return
	}
	line := strings.TrimSuffix(string(cmdlineBytes), "\n")
	arguments := strings.Split(line, " ")
	for _, a := range arguments {
		kv := strings.Split(a, "=")
		if len(kv) == 2 {
			cmdline[kv[0]] = kv[1]
		}
	}

	return cmdline, err
}

func probe() (b []*BlockDevice, err error) {
	b = []*BlockDevice{}

	arguments, err := parseProcCmdline()
	if err != nil {
		return
	}

	if root, ok := arguments[constants.KernelRootFlag]; ok {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			return nil, fmt.Errorf("device does not exist: %s", root)
		}
		pr, err := blkid.NewProbeFromFilename(root)
		defer blkid.FreeProbe(pr)
		if err != nil {
			return nil, fmt.Errorf("failed to probe %s: %s", root, err)
		}

		ls := blkid.ProbeGetPartitions(pr)
		nparts := blkid.ProbeGetPartitionsPartlistNumOfPartitions(ls)

		for i := 0; i < nparts; i++ {
			dev := fmt.Sprintf("%s%d", root, i+1)
			pr, err = blkid.NewProbeFromFilename(dev)
			defer blkid.FreeProbe(pr)
			if err != nil {
				return nil, fmt.Errorf("failed to probe %s: %s", dev, err)
			}

			blkid.DoProbe(pr)
			UUID, err := blkid.ProbeLookupValue(pr, "UUID", nil)
			if err != nil {
				return nil, err
			}
			TYPE, err := blkid.ProbeLookupValue(pr, "TYPE", nil)
			if err != nil {
				return nil, err
			}
			LABEL, err := blkid.ProbeLookupValue(pr, "LABEL", nil)
			if err != nil {
				return nil, err
			}

			b = append(b, &BlockDevice{
				dev:   dev,
				UUID:  UUID,
				TYPE:  TYPE,
				LABEL: LABEL,
			})
		}
	}

	return b, nil
}
