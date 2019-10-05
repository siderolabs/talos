/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/blockdevice"
	"github.com/talos-systems/talos/pkg/blockdevice/filesystem/xfs"
	gptpartition "github.com/talos-systems/talos/pkg/blockdevice/table/gpt/partition"
	"github.com/talos-systems/talos/pkg/blockdevice/util"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/retry"
)

// RetryFunc defines the requirements for retrying a mount point operation.
type RetryFunc func(*Point) error

func mountRetry(f RetryFunc, p *Point) (err error) {
	err = retry.Constant(5*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
		if err = f(p); err != nil {
			switch err {
			case unix.EBUSY:
				return retry.ExpectedError(err)
			default:
				return retry.UnexpectedError(err)
			}
		}

		return nil
	})

	return err
}

// Point represents a Linux mount point.
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
	p.target = path.Join(p.Prefix, p.target)

	if err = ensureDirectory(p.target); err != nil {
		return err
	}

	if p.ReadOnly {
		p.flags |= unix.MS_RDONLY
	}

	switch {
	case p.Overlay:
		err = mountRetry(overlay, p)
	default:
		err = mountRetry(mount, p)
	}

	if err != nil {
		return err
	}

	if p.Shared {
		if err = mountRetry(share, p); err != nil {
			return errors.Errorf("error sharing mount point %s: %+v", p.target, err)
		}
	}

	return nil
}

// Unmount attempts to retry an unmount on EBUSY. It will attempt a
// retry every 100 milliseconds over the course of 5 seconds.
func (p *Point) Unmount() (err error) {
	p.target = path.Join(p.Prefix, p.target)
	if err := mountRetry(unmount, p); err != nil {
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

func mount(p *Point) (err error) {
	return unix.Mount(p.source, p.target, p.fstype, p.flags, p.data)
}

func unmount(p *Point) error {
	return unix.Unmount(p.target, 0)
}

func share(p *Point) error {
	return unix.Mount("", p.target, "", unix.MS_SHARED, "")
}

func overlay(p *Point) error {
	parts := strings.Split(p.target, "/")
	prefix := strings.Join(parts[1:], "-")
	diff := fmt.Sprintf(filepath.Join(constants.SystemVarPath, "%s-diff"), prefix)
	workdir := fmt.Sprintf(filepath.Join(constants.SystemVarPath, "%s-workdir"), prefix)

	for _, target := range []string{diff, workdir} {
		if err := ensureDirectory(target); err != nil {
			return err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", p.target, diff, workdir)
	if err := unix.Mount("overlay", p.target, "overlay", 0, opts); err != nil {
		return errors.Errorf("error creating overlay mount to %s: %v", p.target, err)
	}

	return nil
}

func ensureDirectory(target string) (err error) {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		if err = os.MkdirAll(target, os.ModeDir); err != nil {
			return errors.Errorf("error creating mount point directory %s: %v", target, err)
		}
	}

	return nil
}
