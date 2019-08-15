/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

import (
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
	gptpartition "github.com/talos-systems/talos/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/constants"
	"golang.org/x/sys/unix"
)

// Point represents a linux mount point.
type Point struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string
	*Options
}

// PointMap represents a unique set of mount points.
type PointMap = map[string]*Point

// Points represents an ordered set of mount points.
type Points struct {
	points PointMap
	order  []string
}

// NewMountPoint initializes and returns a Point struct.
func NewMountPoint(source string, target string, fstype string, flags uintptr, data string, setters ...Option) *Point {
	opts := NewDefaultOptions(setters...)
	return &Point{
		source:  source,
		target:  target,
		fstype:  fstype,
		flags:   flags,
		data:    data,
		Options: opts,
	}
}

// NewMountPoints initializes and returns a Points struct.
func NewMountPoints() *Points {
	return &Points{
		points: make(PointMap),
	}
}

// Source returns the mount points source field.
func (p *Point) Source() string {
	return p.source
}

// Target returns the mount points target field.
func (p *Point) Target() string {
	return p.target
}

// Fstype returns the mount points fstype field.
func (p *Point) Fstype() string {
	return p.fstype
}

// Flags returns the mount points flags field.
func (p *Point) Flags() uintptr {
	return p.flags
}

// Data returns the mount points data field.
func (p *Point) Data() string {
	return p.data
}

// Mount attempts to retry a mount on EBUSY. It will attempt a retry
// every 100 milliseconds over the course of 5 seconds.
func (p *Point) Mount() (err error) {
	if p.ReadOnly {
		p.flags |= unix.MS_RDONLY
	}

	target := path.Join(p.Prefix, p.target)

	if err = os.MkdirAll(target, os.ModeDir); err != nil {
		return errors.Errorf("error creating mount point directory %s: %v", target, err)
	}

	retry := func(source string, target string, fstype string, flags uintptr, data string) (err error) {
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

	if err = retry(p.source, target, p.fstype, p.flags, p.data); err != nil {
		return err
	}

	if p.Shared {
		if err = retry("", target, "", unix.MS_SHARED, ""); err != nil {
			return errors.Errorf("error mounting shared mount point %s: %v", target, err)
		}
	}

	return nil

}

// Unmount attempts to retry an unmount on EBUSY. It will attempt a
// retry every 100 milliseconds over the course of 5 seconds.
func (p *Point) Unmount() (err error) {
	retry := func(target string, flags int) error {
		for i := 0; i < 50; i++ {
			if err = unix.Unmount(target, flags); err != nil {
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

	target := path.Join(p.Prefix, p.target)
	if err := retry(target, 0); err != nil {
		return err
	}

	return nil
}

// Move moves a mountpoint to a new location with a prefix.
func (p *Point) Move(prefix string) (err error) {
	target := p.Target()
	mountpoint := NewMountPoint(target, target, "", unix.MS_MOVE, "", WithPrefix(prefix))
	if err = mountpoint.Mount(); err != nil {
		return errors.Errorf("error moving mount point %s: %v", target, err)
	}

	return nil
}

// ResizePartition resizes a partition to the maximum size allowed.
func (p *Point) ResizePartition() (err error) {
	var devname string
	if devname, err = util.DevnameFromPartname(p.Source()); err != nil {
		return err
	}
	bd, err := blockdevice.Open("/dev/" + devname)
	if err != nil {
		return errors.Errorf("error opening block device %q: %v", devname, err)
	}
	// nolint: errcheck
	defer bd.Close()

	pt, err := bd.PartitionTable(true)
	if err != nil {
		return err
	}

	if err := pt.Repair(); err != nil {
		return err
	}

	for _, partition := range pt.Partitions() {
		if partition.(*gptpartition.Partition).Name == constants.EphemeralPartitionLabel {
			if err := pt.Resize(partition); err != nil {
				return err
			}
		}
	}

	if err := pt.Write(); err != nil {
		return err
	}

	// NB: Rereading the partition table requires that all partitions be
	// unmounted or it will fail with EBUSY.
	if err := bd.RereadPartitionTable(); err != nil {
		return err
	}

	return nil
}

// GrowFilesystem grows a partition's filesystem to the maximum size allowed.
// NB: An XFS partition MUST be mounted, or this will fail.
func (p *Point) GrowFilesystem() (err error) {
	if err = xfs.GrowFS(p.Target()); err != nil {
		return errors.Wrap(err, "xfs_growfs")
	}

	return nil
}
