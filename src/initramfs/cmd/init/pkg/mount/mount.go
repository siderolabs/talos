// +build linux

package mount

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/mount/blkid"
	"golang.org/x/sys/unix"
)

var (
	instance struct {
		special      map[string]*Point
		blockdevices map[string]*Point
	}

	once sync.Once

	special = map[string]*Point{
		"dev":  {"devtmpfs", "/dev", "devtmpfs", unix.MS_NOSUID, "mode=0755"},
		"proc": {"proc", "/proc", "proc", unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV, ""},
		"sys":  {"sysfs", "/sys", "sysfs", unix.MS_NOSUID | unix.MS_NOEXEC | unix.MS_NODEV, ""},
		"run":  {"tmpfs", "/run", "tmpfs", 0, ""},
		"tmp":  {"tmpfs", "/tmp", "tmpfs", 0, ""},
	}
)

// Point represents a linux mount point.
type Point struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string
}

// BlockDevice represents the metadata on a block device probed by libblkid.
type BlockDevice struct {
	dev   string
	TYPE  string
	UUID  string
	LABEL string
}

// Init initializes the mount points.
func Init(s string) (err error) {
	once.Do(func() {
		instance = struct {
			special      map[string]*Point
			blockdevices map[string]*Point
		}{
			special,
			map[string]*Point{},
		}
	})

	if err = mountSpecialDevices(); err != nil {
		return
	}
	if err = mountBlockDevices(s); err != nil {
		return
	}

	return nil
}

// Move moves the mount points created in Init, to the new root.
func Move(s string) error {
	if err := os.MkdirAll(s, os.ModeDir); err != nil {
		return err
	}

	// Move the special mounts to the new root.
	for label, mountpoint := range instance.special {
		target := path.Join(s, mountpoint.target)
		if err := unix.Mount(mountpoint.target, target, "", unix.MS_MOVE, ""); err != nil {
			return fmt.Errorf("move mount point %s to %s: %s", mountpoint.target, target, err.Error())
		}
		if label == "dev" {
			mountpoint = &Point{"devpts", path.Join(s, "/dev/pts"), "devpts", unix.MS_NOSUID | unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"}
			if err := os.MkdirAll(mountpoint.target, os.ModeDir); err != nil {
				return fmt.Errorf("create %s: %s", mountpoint.target, err.Error())
			}
			if err := unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
				return fmt.Errorf("mount %s: %s", mountpoint.target, err.Error())
			}
		}
	}

	return nil
}

// Finalize moves the mount points created in Init to the new root.
func Finalize(s string) error {
	return unix.Mount(s, "/", "", unix.MS_MOVE, "")
}

// Mount moves the mount points created in Init to the new root.
func Mount(s string) error {
	if err := os.MkdirAll(s, os.ModeDir); err != nil {
		return err
	}

	mountpoint, ok := instance.blockdevices[constants.RootPartitionLabel]
	if ok {
		mountpoint.flags = unix.MS_RDONLY | unix.MS_NOATIME
		if err := unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("mount %s: %s", mountpoint.target, err.Error())
		}
		// MS_SHARED:
		//   Make this mount point shared.  Mount and unmount events
		//   immediately under this mount point will propagate to the
		//   other mount points that are members of this mount's peer
		//   group. Propagation here means that the same mount or
		//   unmount will automatically occur under all of the other
		//   mount points in the peer group.  Conversely, mount and
		//   unmount events that take place under peer mount points
		//   will propagate to this mount point.
		// See http://man7.org/linux/man-pages/man2/mount.2.html
		// https://github.com/kubernetes/kubernetes/issues/61058
		if err := unix.Mount("", mountpoint.target, "", unix.MS_SHARED, ""); err != nil {
			return fmt.Errorf("mount %s as shared: %s", mountpoint.target, err.Error())
		}
	}
	mountpoint, ok = instance.blockdevices[constants.DataPartitionLabel]
	if ok {
		if err := unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("mount %s: %s", mountpoint.target, err.Error())
		}
	}

	return nil
}

// Unmount unmounts the ROOT and DATA block devices.
func Unmount() error {
	mountpoint, ok := instance.blockdevices[constants.DataPartitionLabel]
	if ok {
		if err := unix.Unmount(mountpoint.target, 0); err != nil {
			return fmt.Errorf("unmount mount point %s: %s", mountpoint.target, err.Error())
		}
	}
	mountpoint, ok = instance.blockdevices[constants.RootPartitionLabel]
	if ok {
		if err := unix.Unmount(mountpoint.target, 0); err != nil {
			return fmt.Errorf("unmount mount point %s: %s", mountpoint.target, err.Error())
		}
	}

	return nil
}

func mountSpecialDevices() (err error) {
	for _, mountpoint := range instance.special {
		if err = os.MkdirAll(mountpoint.target, os.ModeDir); err != nil {
			return fmt.Errorf("create %s: %s", mountpoint.target, err.Error())
		}
		if err = unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("mount %s: %s", mountpoint.target, err.Error())
		}
	}

	return nil
}

func mountBlockDevices(s string) (err error) {
	probed, err := probe()
	if err != nil {
		return fmt.Errorf("probe block devices: %s", err.Error())
	}
	for _, b := range probed {
		mountpoint := &Point{
			source: b.dev,
			fstype: b.TYPE,
			flags:  unix.MS_NOATIME,
			data:   "",
		}
		switch b.LABEL {
		case constants.RootPartitionLabel:
			mountpoint.target = s
		case constants.DataPartitionLabel:
			mountpoint.target = path.Join(s, "var")
		default:
			continue
		}

		if err = os.MkdirAll(mountpoint.target, os.ModeDir); err != nil {
			return fmt.Errorf("create %s: %s", mountpoint.target, err.Error())
		}
		if err = unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("mount %s: %s", mountpoint.target, err.Error())
		}

		instance.blockdevices[b.LABEL] = mountpoint
	}

	return nil
}

func probe() (b []*BlockDevice, err error) {
	b = []*BlockDevice{}

	if err := appendBlockDeviceWithLabel(&b, constants.RootPartitionLabel); err != nil {
		return nil, err
	}
	if err := appendBlockDeviceWithLabel(&b, constants.DataPartitionLabel); err != nil {
		return nil, err
	}

	return b, nil
}

func appendBlockDeviceWithLabel(b *[]*BlockDevice, value string) error {
	devname, err := blkid.GetDevWithAttribute("LABEL", value)
	if err != nil {
		return err
	}

	blockDevice, err := probeDevice(devname)
	if err != nil {
		return err
	}

	*b = append(*b, blockDevice)

	return nil
}

func probeDevice(devname string) (*BlockDevice, error) {
	pr, err := blkid.NewProbeFromFilename(devname)
	defer blkid.FreeProbe(pr)
	if err != nil {
		return nil, fmt.Errorf("failed to probe %s: %s", devname, err)
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

	return &BlockDevice{
		dev:   devname,
		UUID:  UUID,
		TYPE:  TYPE,
		LABEL: LABEL,
	}, nil
}
