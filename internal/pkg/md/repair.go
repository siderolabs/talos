// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package md

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	// mdSectorSize is the unit of the md sysfs sync attributes, regardless of the logical block size.
	mdSectorSize = 512

	// syncWindowAlign is the alignment of sync_min: the kernel rounds it down to 4KiB.
	syncWindowAlign = 8

	// repairSpeedMin is the sync_speed_min (KiB/s) used for the duration of a range repair.
	//
	// The default floor is 1000 KiB/s, and md throttles down to it whenever the members see other
	// I/O, which is normal on a running node: that would turn a repair of a few hundred MiB into
	// minutes.
	repairSpeedMin = "100000"

	// restoreTimeout bounds stopping the repair and restoring the sync window, which runs even if
	// the context of the repair is canceled.
	restoreTimeout = 30 * time.Second

	// busyRetryTimeout bounds the retries of a sync_min write which is refused with EBUSY.
	busyRetryTimeout = 5 * time.Second

	// repairPollInterval is how often the sync progress is polled.
	repairPollInterval = 100 * time.Millisecond
)

// IsArray reports whether the device (which might be a symlink, e.g. a by-id path) is an MD array.
func (md *MD) IsArray(device string) (bool, error) {
	resolved, err := filepath.EvalSymlinks(device)
	if err != nil {
		return false, fmt.Errorf("failed to resolve %s: %w", device, err)
	}

	_, err = os.Stat(filepath.Join(md.sysBlockDir, filepath.Base(resolved), "md"))

	switch {
	case err == nil:
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, err
	}
}

// RepairRange runs an MD "repair" pass over the byte range [offset, offset+length) of a RAID1 array.
//
// It returns the number of sectors which were found to differ between the members (mismatch_cnt): RAID1
// repair rewrites them on every member from the first one which read the block successfully.
//
// The device might be a symlink to the /dev/mdN node. The range is relative to the array itself, so it
// doesn't depend on the metadata format (data offset on the members).
//
// The range is set via sync_min/sync_max: the kernel starts a requested repair at sync_min, and pauses
// (rather than finishes) once it reaches sync_max. So the pass is stopped by writing "idle" once
// sync_completed reports the end of the range, and sync_min, sync_max and sync_speed_min are restored
// to their original values afterwards, even if the repair fails.
//
// The array has to be idle: a repair can't be started while any other sync operation is running. The
// exception is a range repair which was left paused at sync_max, as the process which requested it died
// before stopping it: that one is stopped, and the sync window is reset to the defaults.
//
//nolint:gocyclo,cyclop
func (md *MD) RepairRange(ctx context.Context, device string, offset, length uint64) (mismatches uint64, err error) {
	if device == "" || length == 0 {
		return 0, fmt.Errorf("%w: device and a non-empty range must be set", ErrInvalidArgument)
	}

	resolved, err := filepath.EvalSymlinks(device)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve %s: %w", device, err)
	}

	name := filepath.Base(resolved)
	attrs := mdAttrs(filepath.Join(md.sysBlockDir, name, "md"))

	level, err := attrs.read("level")
	if err != nil {
		return 0, err
	}

	if level != "raid1" {
		return 0, fmt.Errorf("%w: range repair of %s is not supported for level %q", ErrInvalidArgument, name, level)
	}

	action, err := attrs.read("sync_action")
	if err != nil {
		return 0, err
	}

	if SyncAction(action) == SyncActionRepair {
		var stale bool

		if stale, err = attrs.pausedRepair(); err != nil {
			return 0, err
		}

		if stale {
			if err = attrs.restore("0", "max", "system"); err != nil {
				return 0, fmt.Errorf("failed to reset the stale range repair of %s: %w", name, err)
			}

			if action, err = attrs.read("sync_action"); err != nil {
				return 0, err
			}
		}
	}

	if SyncAction(action) != SyncActionIdle {
		return 0, fmt.Errorf("%w: %s is busy with %q", ErrResync, name, action)
	}

	sizeRaw, err := os.ReadFile(filepath.Join(md.sysBlockDir, name, "size"))
	if err != nil {
		return 0, fmt.Errorf("failed to read size of %s: %w", name, err)
	}

	size, err := strconv.ParseUint(strings.TrimSpace(string(sizeRaw)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size of %s: %w", name, err)
	}

	start := offset / mdSectorSize / syncWindowAlign * syncWindowAlign
	end := (offset + length + mdSectorSize*syncWindowAlign - 1) / (mdSectorSize * syncWindowAlign) * syncWindowAlign

	// sync_max has to stay below the end of the array: the repair would finish there instead of pausing,
	// and there would be no way to tell that apart from an interrupted repair.
	if end >= size {
		return 0, fmt.Errorf("%w: range [%d, %d) is not within the array %s of %d sectors", ErrInvalidArgument, start, end, name, size)
	}

	origMin, err := attrs.read("sync_min")
	if err != nil {
		return 0, err
	}

	origMax, err := attrs.read("sync_max")
	if err != nil {
		return 0, err
	}

	origSpeedMin, err := attrs.read("sync_speed_min")
	if err != nil {
		return 0, err
	}

	// sync_speed_min reads as "<value> (system)" or "<value> (local)", but only takes a value or "system"
	if value, source, ok := strings.Cut(origSpeedMin, " "); ok && source == "(local)" {
		origSpeedMin = value
	} else {
		origSpeedMin = "system"
	}

	defer func() {
		err = errors.Join(err, attrs.restore(origMin, origMax, origSpeedMin))
	}()

	for _, step := range []struct {
		attr, value string
	}{
		{"sync_speed_min", repairSpeedMin},
		// lift sync_max first, as sync_min can't be set above it
		{"sync_max", "max"},
		{"sync_min", strconv.FormatUint(start, 10)},
		{"sync_max", strconv.FormatUint(end, 10)},
		{"sync_action", string(SyncActionRepair)},
	} {
		write := attrs.write
		if step.attr == "sync_min" {
			write = attrs.writeRetryBusy
		}

		if err = write(step.attr, step.value); err != nil {
			return 0, err
		}
	}

	for {
		completed, err := attrs.read("sync_completed")
		if err != nil {
			return 0, err
		}

		// the action is read after the progress: if it is still the repair, the progress is the one of
		// the repair, and not of e.g. a recovery which the kernel started instead, as a member was added
		// to the array just before the repair was requested (it would pause at sync_max as well)
		action, err := attrs.read("sync_action")
		if err != nil {
			return 0, err
		}

		if SyncAction(action) == SyncActionIdle {
			return 0, fmt.Errorf("repair of %s stopped before reaching sector %d", name, end)
		}

		if SyncAction(action) != SyncActionRepair {
			return 0, fmt.Errorf("%w: repair of %s was replaced with %q", ErrResync, name, action)
		}

		if pos, ok := syncPosition(completed); ok && pos >= end {
			break
		}

		select {
		case <-ctx.Done():
			return 0, fmt.Errorf("repair of %s was not finished: %w", name, ctx.Err())
		case <-time.After(repairPollInterval):
		}
	}

	mismatchesRaw, err := attrs.read("mismatch_cnt")
	if err != nil {
		return 0, err
	}

	mismatches, err = strconv.ParseUint(mismatchesRaw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse mismatch_cnt of %s: %w", name, err)
	}

	return mismatches, nil
}

type mdAttrs string

func (a mdAttrs) read(attr string) (string, error) {
	out, err := os.ReadFile(filepath.Join(string(a), attr))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}

		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

func (a mdAttrs) write(attr, value string) error {
	if err := os.WriteFile(filepath.Join(string(a), attr), []byte(value), 0); err != nil {
		return fmt.Errorf("failed to set md %s to %q: %w", attr, value, err)
	}

	return nil
}

// writeRetryBusy writes an attribute which can only be changed while no sync thread is running.
//
// The kernel marks the sync as running for a moment each time it checks whether there is something to
// sync (e.g. right after "idle" is written to sync_action), while sync_action still reads "idle": the
// write is refused with EBUSY, so it is retried.
func (a mdAttrs) writeRetryBusy(attr, value string) error {
	deadline := time.Now().Add(busyRetryTimeout)

	for {
		err := a.write(attr, value)
		if err == nil || !errors.Is(err, unix.EBUSY) || time.Now().After(deadline) {
			return err
		}

		time.Sleep(repairPollInterval)
	}
}

// syncPosition parses sync_completed: "<position> / <max>" while a sync runs, "none" or "delayed" otherwise.
func syncPosition(completed string) (uint64, bool) {
	position, _, ok := strings.Cut(completed, " / ")
	if !ok {
		return 0, false
	}

	pos, err := strconv.ParseUint(position, 10, 64)

	return pos, err == nil
}

// pausedRepair reports whether the sync in progress is paused at a sync_max which was set below the end
// of the array.
func (a mdAttrs) pausedRepair() (bool, error) {
	syncMax, err := a.read("sync_max")
	if err != nil {
		return false, err
	}

	limit, err := strconv.ParseUint(syncMax, 10, 64)
	if err != nil {
		// "max": the sync is not limited to a range
		return false, nil //nolint:nilerr
	}

	completed, err := a.read("sync_completed")
	if err != nil {
		return false, err
	}

	pos, ok := syncPosition(completed)

	return ok && pos >= limit, nil
}

// restore stops a (paused) repair and restores the sync window and the speed floor.
func (a mdAttrs) restore(origMin, origMax, origSpeedMin string) error {
	deadline := time.Now().Add(restoreTimeout)

	// sync_min can't be changed while the sync thread is running
	for attempt := 0; ; attempt++ {
		action, err := a.read("sync_action")
		if err != nil {
			return err
		}

		if SyncAction(action) == SyncActionIdle {
			break
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for the md sync to stop, sync_action is %q", action)
		}

		if attempt > 0 {
			time.Sleep(repairPollInterval)
		}

		// "idle" stops a running sync before the write returns, but it doesn't cancel a sync which was
		// requested and not started yet: that one starts afterwards, so "idle" is written until it sticks
		if err = a.write("sync_action", string(SyncActionIdle)); err != nil {
			return err
		}
	}

	// sync_max goes first, as sync_min can't be set above it (an interrupted repair leaves sync_min at
	// the position it reached)
	return errors.Join(
		a.write("sync_max", origMax),
		a.writeRetryBusy("sync_min", origMin),
		a.write("sync_speed_min", origSpeedMin),
	)
}
