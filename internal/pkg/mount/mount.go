/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

import (
	"os"
	"path"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// Point represents a linux mount point.
type Point struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string
}

// PointMap represents a unique set of mount points.
type PointMap = map[string]*Point

// Points represents an ordered set of mount points.
type Points struct {
	points PointMap
	order  []string
}

// NewMountPoint initializes and returns a Point struct.
func NewMountPoint(source string, target string, fstype string, flags uintptr, data string) *Point {
	return &Point{
		source: source,
		target: target,
		fstype: fstype,
		flags:  flags,
		data:   data,
	}
}

// NewMountPoints initializes and returns a Points struct.
func NewMountPoints() *Points {
	return &Points{
		points: make(PointMap, 0),
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

// WithRetry attempts to retry a mount on EBUSY. It will attempt a retry
// every 100 milliseconds over the course of 5 seconds.
func WithRetry(mountpoint *Point, setters ...Option) (err error) {
	opts := NewDefaultOptions(setters...)

	if opts.ReadOnly {
		mountpoint.flags |= unix.O_RDONLY
	}

	target := path.Join(opts.Prefix, mountpoint.target)
	if err = os.MkdirAll(target, os.ModeDir); err != nil {
		return errors.Errorf("error creating mount point directory %s: %v", target, err)
	}

	retry := func(source string, target string, fstype string, flags uintptr, data string) error {
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

	if err = retry(mountpoint.source, target, mountpoint.fstype, mountpoint.flags, mountpoint.data); err != nil {
		return err
	}

	if opts.Shared {
		if err = retry("", target, "", unix.MS_SHARED, ""); err != nil {
			return errors.Errorf("error mounting shared mount point %s: %v", target, err)
		}
	}

	return nil

}

// UnWithRetry attempts to retry an unmount on EBUSY. It will attempt a
// retry every 100 milliseconds over the course of 5 seconds.
func UnWithRetry(mountpoint *Point, setters ...Option) (err error) {
	opts := NewDefaultOptions(setters...)

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

	target := path.Join(opts.Prefix, mountpoint.target)
	if err := retry(target, 0); err != nil {
		return err
	}

	return nil
}
