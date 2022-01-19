// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/filesystem"
	"github.com/talos-systems/go-blockdevice/blockdevice/util"
	"github.com/talos-systems/go-retry/retry"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/makefs"
)

// RetryFunc defines the requirements for retrying a mount point operation.
type RetryFunc func(*Point) error

// Mount mounts the device(s).
func Mount(mountpoints *Points) (err error) {
	iter := mountpoints.Iter()

	//  Mount the device(s).

	for iter.Next() {
		if _, err = mountMountpoint(iter.Value()); err != nil {
			return fmt.Errorf("error mounting %q: %w", iter.Value().Source(), err)
		}
	}

	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

//nolint:gocyclo
func mountMountpoint(mountpoint *Point) (skipMount bool, err error) {
	// Repair the disk's partition table.
	if mountpoint.MountFlags.Check(Resize) {
		if _, err = mountpoint.ResizePartition(); err != nil {
			return false, fmt.Errorf("error resizing %w", err)
		}
	}

	if mountpoint.MountFlags.Check(SkipIfMounted) {
		skipMount, err = mountpoint.IsMounted()
		if err != nil {
			return false, fmt.Errorf("mountpoint is set to skip if mounted, but the mount check failed: %w", err)
		}
	}

	if mountpoint.MountFlags.Check(SkipIfNoFilesystem) && mountpoint.Fstype() == filesystem.Unknown {
		skipMount = true
	}

	if !skipMount {
		if err = mountpoint.Mount(); err != nil {
			return false, fmt.Errorf("error mounting: %w", err)
		}
	}

	// Grow the filesystem to the maximum allowed size.
	//
	// Growfs is called always, even if ResizePartition returns false to workaround failure scenario
	// when partition was resized, but growfs never got called.
	if mountpoint.MountFlags.Check(Resize) {
		if err = mountpoint.GrowFilesystem(); err != nil {
			return false, fmt.Errorf("error resizing filesystem: %w", err)
		}
	}

	return skipMount, nil
}

// Unmount unmounts the device(s).
func Unmount(mountpoints *Points) (err error) {
	iter := mountpoints.IterRev()
	for iter.Next() {
		mountpoint := iter.Value()
		if err = mountpoint.Unmount(); err != nil {
			return fmt.Errorf("unmount: %w", err)
		}
	}

	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// Move moves the device(s).
// TODO(andrewrynhard): We need to skip calling the move method on mountpoints
// that are a child of another mountpoint. The kernel will handle moving the
// child mountpoints for us.
func Move(mountpoints *Points, prefix string) (err error) {
	iter := mountpoints.Iter()
	for iter.Next() {
		mountpoint := iter.Value()
		if err = mountpoint.Move(prefix); err != nil {
			return fmt.Errorf("move: %w", err)
		}
	}

	if iter.Err() != nil {
		return iter.Err()
	}

	return nil
}

// PrefixMountTargets prefixes all mountpoints targets with fixed path.
func PrefixMountTargets(mountpoints *Points, targetPrefix string) error {
	iter := mountpoints.Iter()
	for iter.Next() {
		mountpoint := iter.Value()
		mountpoint.target = filepath.Join(targetPrefix, mountpoint.target)
	}

	return iter.Err()
}

func mountRetry(f RetryFunc, p *Point, isUnmount bool) (err error) {
	err = retry.Constant(5*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(func() error {
		if err = f(p); err != nil {
			switch err {
			case unix.EBUSY:
				return retry.ExpectedError(err)
			case unix.ENOENT:
				// if udevd triggers BLKRRPART ioctl, partition device entry might disappear temporarily
				return retry.ExpectedError(err)
			case unix.EINVAL:
				isMounted, checkErr := p.IsMounted()
				if checkErr != nil {
					return retry.ExpectedError(checkErr)
				}

				if !isMounted && isUnmount { // if partition is already unmounted, ignore EINVAL
					return nil
				}

				return err
			default:
				return err
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
func NewMountPoint(source, target, fstype string, flags uintptr, data string, setters ...Option) *Point {
	opts := NewDefaultOptions(setters...)

	p := &Point{
		source:  source,
		target:  target,
		fstype:  fstype,
		flags:   flags,
		data:    data,
		Options: opts,
	}

	if p.Prefix != "" {
		p.target = filepath.Join(p.Prefix, p.target)
	}

	return p
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
	for _, hook := range p.Options.PreMountHooks {
		if err = hook(p); err != nil {
			return err
		}
	}

	if err = ensureDirectory(p.target); err != nil {
		return err
	}

	if p.MountFlags.Check(ReadOnly) {
		p.flags |= unix.MS_RDONLY
	}

	switch {
	case p.MountFlags.Check(Overlay):
		err = mountRetry(overlay, p, false)
	case p.MountFlags.Check(ReadonlyOverlay):
		err = mountRetry(readonlyOverlay, p, false)
	default:
		err = mountRetry(mount, p, false)
	}

	if err != nil {
		return err
	}

	if p.MountFlags.Check(Shared) {
		if err = mountRetry(share, p, false); err != nil {
			return fmt.Errorf("error sharing mount point %s: %+v", p.target, err)
		}
	}

	return nil
}

// Unmount attempts to retry an unmount on EBUSY. It will attempt a
// retry every 100 milliseconds over the course of 5 seconds.
func (p *Point) Unmount() (err error) {
	var mounted bool

	if mounted, err = p.IsMounted(); err != nil {
		return err
	}

	if mounted {
		if err = mountRetry(unmount, p, true); err != nil {
			return err
		}
	}

	for _, hook := range p.Options.PostUnmountHooks {
		if err = hook(p); err != nil {
			return err
		}
	}

	return nil
}

// IsMounted checks whether mount point is active under /proc/mounts.
func (p *Point) IsMounted() (bool, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false, err
	}

	defer f.Close() //nolint:errcheck

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 2 {
			continue
		}

		mountpoint := fields[1]

		if mountpoint == p.target {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// Move moves a mountpoint to a new location with a prefix.
func (p *Point) Move(prefix string) (err error) {
	target := p.Target()
	mountpoint := NewMountPoint(target, target, "", unix.MS_MOVE, "", WithPrefix(prefix))

	if err = mountpoint.Mount(); err != nil {
		return fmt.Errorf("error moving mount point %s: %w", target, err)
	}

	return nil
}

// ResizePartition resizes a partition to the maximum size allowed.
func (p *Point) ResizePartition() (resized bool, err error) {
	var devname string

	if devname, err = util.DevnameFromPartname(p.Source()); err != nil {
		return false, err
	}

	bd, err := blockdevice.Open("/dev/"+devname, blockdevice.WithExclusiveLock(true))
	if err != nil {
		return false, fmt.Errorf("error opening block device %q: %w", devname, err)
	}

	//nolint:errcheck
	defer bd.Close()

	pt, err := bd.PartitionTable()
	if err != nil {
		return false, err
	}

	if err := pt.Repair(); err != nil {
		return false, err
	}

	for _, partition := range pt.Partitions().Items() {
		if partition.Name == constants.EphemeralPartitionLabel {
			resized, err := pt.Resize(partition)
			if err != nil {
				return false, err
			}

			if !resized {
				return false, nil
			}
		}
	}

	if err := pt.Write(); err != nil {
		return false, err
	}

	return true, nil
}

// GrowFilesystem grows a partition's filesystem to the maximum size allowed.
// NB: An XFS partition MUST be mounted, or this will fail.
func (p *Point) GrowFilesystem() (err error) {
	if err = makefs.XFSGrow(p.Target()); err != nil {
		return fmt.Errorf("xfs_growfs: %w", err)
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
	return unix.Mount("", p.target, "", unix.MS_SHARED|unix.MS_REC, "")
}

func overlay(p *Point) error {
	parts := strings.Split(p.target, "/")
	prefix := strings.Join(parts[1:], "-")
	diff := fmt.Sprintf(filepath.Join(constants.SystemOverlaysPath, "%s-diff"), prefix)
	workdir := fmt.Sprintf(filepath.Join(constants.SystemOverlaysPath, "%s-workdir"), prefix)

	for _, target := range []string{diff, workdir} {
		if err := ensureDirectory(target); err != nil {
			return err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", p.target, diff, workdir)
	if err := unix.Mount("overlay", p.target, "overlay", 0, opts); err != nil {
		return fmt.Errorf("error creating overlay mount to %s: %w", p.target, err)
	}

	return nil
}

func readonlyOverlay(p *Point) error {
	opts := fmt.Sprintf("lowerdir=%s", p.source)
	if err := unix.Mount("overlay", p.target, "overlay", p.flags, opts); err != nil {
		return fmt.Errorf("error creating overlay mount to %s: %w", p.target, err)
	}

	return nil
}

func ensureDirectory(target string) (err error) {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		if err = os.MkdirAll(target, os.ModeDir); err != nil {
			return fmt.Errorf("error creating mount point directory %s: %w", target, err)
		}
	}

	return nil
}
