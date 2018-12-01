// +build linux

package mount

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/fs/xfs"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/mount/blkid"
	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice"
	gptpartition "github.com/autonomy/talos/src/initramfs/pkg/blockdevice/table/gpt/partition"
	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

var (
	instance struct {
		special      map[string]*Point
		blockdevices map[string]*Point
	}

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
	dev             string
	TYPE            string
	UUID            string
	LABEL           string
	PART_ENTRY_NAME string
	PART_ENTRY_UUID string
}

// init initializes the instance metadata
func init() {
	instance = struct {
		special      map[string]*Point
		blockdevices map[string]*Point
	}{
		special,
		map[string]*Point{},
	}
}

// InitSpecial initializes the special device  mount points.
func InitSpecial(s string) (err error) {
	return mountSpecialDevices()
}

// InitBlock initializes the block device mount points.
func InitBlock(s string) (err error) {
	blockdevices, err := probe()
	if err != nil {
		return fmt.Errorf("error probing block devices: %v", err)
	}
	if err = mountBlockDevices(blockdevices, s); err != nil {
		return fmt.Errorf("error mounting partitions: %v", err)
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
		if err := UnixMountWithRetry(mountpoint.target, target, "", unix.MS_MOVE, ""); err != nil {
			return fmt.Errorf("move mount point %s to %s: %v", mountpoint.target, target, err)
		}
		if label == "dev" {
			mountpoint = &Point{"devpts", path.Join(s, "/dev/pts"), "devpts", unix.MS_NOSUID | unix.MS_NOEXEC, "ptmxmode=000,mode=620,gid=5"}
			if err := os.MkdirAll(mountpoint.target, os.ModeDir); err != nil {
				return fmt.Errorf("error creating mount point directory %s: %v", mountpoint.target, err)
			}
			if err := UnixMountWithRetry(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
				return fmt.Errorf("error moving special device from %s to %s: %v", mountpoint.source, mountpoint.target, err)
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
			return fmt.Errorf("error mounting partition %s: %v", mountpoint.target, err)
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
			return fmt.Errorf("error making making mount point %s shared: %v", mountpoint.target, err)
		}
	}
	mountpoint, ok = instance.blockdevices[constants.DataPartitionLabel]
	if ok {
		if err := unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("error mounting partition %s: %v", mountpoint.target, err)
		}
	}

	return nil
}

// Unmount unmounts the ROOT and DATA block devices.
func Unmount() error {
	for _, disk := range []string{constants.RootPartitionLabel, constants.DataPartitionLabel} {
		mountpoint, ok := instance.blockdevices[disk]
		if ok {
			if err := unix.Unmount(mountpoint.target, 0); err != nil {
				return fmt.Errorf("unmount mount point %s: %v", mountpoint.target, err)
			}
		}
	}

	return nil
}

func mountSpecialDevices() (err error) {
	for _, mountpoint := range instance.special {
		if err = os.MkdirAll(mountpoint.target, os.ModeDir); err != nil {
			return fmt.Errorf("error creating mount point directory %s: %v", mountpoint.target, err)
		}
		if err = UnixMountWithRetry(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("error mounting special device %s: %v", mountpoint.target, err)
		}
	}

	return nil
}

// UnixMountWithRetry attempts to retry a mount on EBUSY. It will attempt a
// retry every 100 milliseconds over the course of 5 seconds.
func UnixMountWithRetry(source string, target string, fstype string, flags uintptr, data string) (err error) {
	for i := 0; i < 50; i++ {
		if err = unix.Mount(source, target, fstype, flags, data); err != nil {
			switch err {
			case unix.EBUSY:
				time.Sleep(100 * time.Millisecond)
				continue
			default:
				return err
			}
		}
		return nil
	}

	return errors.Errorf("mount timeout: %v", err)
}

// nolint: gocyclo
func fixDataPartition(blockdevices []*BlockDevice) error {
	for _, b := range blockdevices {
		if b.PART_ENTRY_NAME == constants.DataPartitionLabel {
			devname := devnameFromPartname(b.dev)
			bd, err := blockdevice.Open(devname)
			if err != nil {
				return fmt.Errorf("error opening block device %q: %v", devname, err)
			}
			// nolint: errcheck
			defer bd.Close()

			pt, err := bd.PartitionTable(false)
			if err != nil {
				return err
			}

			if err := pt.Read(); err != nil {
				return err
			}

			if err := pt.Repair(); err != nil {
				return err
			}

			for _, partition := range pt.Partitions() {
				if partition.(*gptpartition.Partition).Name == constants.DataPartitionLabel {
					if err := pt.Resize(partition); err != nil {
						return err
					}
				}
			}

			if err := pt.Write(); err != nil {
				return err
			}

			// Rereading the partition table requires that all partitions be unmounted
			// or it will fail with EBUSY.
			if err := bd.RereadPartitionTable(); err != nil {
				return err
			}
		}
	}

	return nil
}

func mountBlockDevices(blockdevices []*BlockDevice, s string) (err error) {
	if err = fixDataPartition(blockdevices); err != nil {
		return fmt.Errorf("error fixing data partition: %v", err)
	}
	for _, b := range blockdevices {
		mountpoint := &Point{
			source: b.dev,
			fstype: b.TYPE,
			flags:  unix.MS_NOATIME,
			data:   "",
		}
		switch b.PART_ENTRY_NAME {
		case constants.RootPartitionLabel:
			mountpoint.target = s
		case constants.DataPartitionLabel:
			mountpoint.target = path.Join(s, "var")
		default:
			continue
		}

		if err = os.MkdirAll(mountpoint.target, os.ModeDir); err != nil {
			return fmt.Errorf("error creating mount point directory %s: %v", mountpoint.target, err)
		}
		if err = unix.Mount(mountpoint.source, mountpoint.target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
			return fmt.Errorf("error mounting partition %s: %v", mountpoint.target, err)
		}

		if b.PART_ENTRY_NAME == constants.DataPartitionLabel {
			// The XFS partition MUST be mounted, or this will fail.
			if err = xfs.GrowFS(mountpoint.target); err != nil {
				return fmt.Errorf("error growing XFS file system: %v", err)
			}
		}

		instance.blockdevices[b.PART_ENTRY_NAME] = mountpoint
	}

	return nil
}

func probe() (b []*BlockDevice, err error) {
	b = []*BlockDevice{}

	for _, disk := range []string{constants.RootPartitionLabel, constants.DataPartitionLabel} {
		if err := appendBlockDeviceWithLabel(&b, disk); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func appendBlockDeviceWithLabel(b *[]*BlockDevice, value string) error {
	devname, err := blkid.GetDevWithAttribute("PART_ENTRY_NAME", value)
	if err != nil {
		return fmt.Errorf("failed to get dev with attribute: %v", err)
	}

	if devname == "" {
		return fmt.Errorf("no device with attribute \"PART_ENTRY_NAME=%s\" found", value)
	}

	blockDevice, err := ProbeDevice(devname)
	if err != nil {
		return fmt.Errorf("failed to probe block device %q: %v", devname, err)
	}

	*b = append(*b, blockDevice)

	return nil
}

// ProbeDevice looks up UUID/TYPE/LABEL/PART_ENTRY_NAME/PART_ENTRY_UUID from a block device
func ProbeDevice(devname string) (*BlockDevice, error) {
	pr, err := blkid.NewProbeFromFilename(devname)
	defer blkid.FreeProbe(pr)
	if err != nil {
		return nil, fmt.Errorf("failed to probe %s: %s", devname, err)
	}
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
	PART_ENTRY_NAME, err := blkid.ProbeLookupValue(pr, "PART_ENTRY_NAME", nil)
	if err != nil {
		return nil, err
	}
	PART_ENTRY_UUID, err := blkid.ProbeLookupValue(pr, "PART_ENTRY_UUID", nil)
	if err != nil {
		return nil, err
	}

	return &BlockDevice{
		dev:             devname,
		UUID:            UUID,
		TYPE:            TYPE,
		LABEL:           LABEL,
		PART_ENTRY_NAME: PART_ENTRY_NAME,
		PART_ENTRY_UUID: PART_ENTRY_UUID,
	}, nil
}

// TODO(andrewrynhard): Should we return an error here?
func partNo(partname string) string {
	if strings.HasPrefix(partname, "/dev/nvme") {
		idx := strings.Index(partname, "p")
		return partname[idx+1:]
	} else if strings.HasPrefix(partname, "/dev/sd") || strings.HasPrefix(partname, "/dev/hd") || strings.HasPrefix(partname, "/dev/vd") {
		return strings.TrimLeft(partname, "/abcdefghijklmnopqrstuvwxyz")
	}

	return ""
}

// TODO(andrewrynhard): Should we return an error here?
func devnameFromPartname(partname string) string {
	partno := partNo(partname)
	if strings.HasPrefix(partname, "/dev/nvme") {
		return strings.TrimRight(partname, "p"+partno)
	} else if strings.HasPrefix(partname, "/dev/sd") || strings.HasPrefix(partname, "/dev/hd") || strings.HasPrefix(partname, "/dev/vd") {
		return strings.TrimRight(partname, partno)
	}

	return ""
}
