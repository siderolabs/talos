// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bootpartition implements the bounded wait for the boot partition to be discovered.
//
// The machine was booted from the boot partition, so the disk holding it certainly exists, it might only be slow
// to enumerate. Until the boot partition is discovered (or the wait times out), the volumes are kept waiting, so
// that the system volumes (META, STATE) located on the same disk are not declared missing prematurely (which
// would send the machine to maintenance mode).
package bootpartition

import (
	"iter"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DefaultWaitTimeout is the default bound on waiting for the boot partition to be discovered.
//
// Some storage controllers (hardware RAID, SAS HBAs) enumerate their disks tens of seconds after
// the driver is loaded, with no way to detect that the scan is still in progress.
const DefaultWaitTimeout = 30 * time.Second

// Waiter tracks the bounded wait for the boot partition to be discovered.
//
// The wait starts on the first call to Discovered with the boot partition not discovered, and ends when the
// boot partition is discovered, or the timeout expires: either way, the wait is done once, and never restarts.
type Waiter struct {
	timeout  time.Duration
	deadline time.Time
	timer    *time.Timer
	done     bool
}

// NewWaiter creates a Waiter with the given timeout (DefaultWaitTimeout if zero).
func NewWaiter(timeout time.Duration) *Waiter {
	if timeout <= 0 {
		timeout = DefaultWaitTimeout
	}

	return &Waiter{timeout: timeout}
}

// TimerC returns the channel to wake up the caller when the wait times out (nil if not waiting).
func (w *Waiter) TimerC() <-chan time.Time {
	if w.timer == nil {
		return nil
	}

	return w.timer.C
}

// Stop releases the timer.
func (w *Waiter) Stop() {
	if w.timer != nil {
		w.timer.Stop()
	}
}

// Discovered reports whether the volumes might be declared missing with respect to the boot partition.
//
// The boot partition UUID is empty if the boot partition is not known (nothing to wait for), otherwise it is
// matched (case-insensitively) against the partition UUIDs of the discovered volumes.
func (w *Waiter) Discovered(logger *zap.Logger, partitionUUID string, discoveredPartitionUUIDs iter.Seq[string]) bool {
	if w.done {
		return true
	}

	if partitionUUID == "" {
		return true
	}

	for discovered := range discoveredPartitionUUIDs {
		if strings.EqualFold(discovered, partitionUUID) {
			logger.Info("boot partition discovered", zap.String("partition_uuid", partitionUUID))

			w.done = true

			return true
		}
	}

	if w.timer == nil {
		logger.Info("waiting for the boot partition to be discovered", zap.String("partition_uuid", partitionUUID), zap.Duration("timeout", w.timeout))

		w.deadline = time.Now().Add(w.timeout)
		w.timer = time.NewTimer(w.timeout)

		return false
	}

	if time.Now().Before(w.deadline) {
		return false
	}

	logger.Warn("boot partition was not discovered, proceeding without it", zap.String("partition_uuid", partitionUUID), zap.Duration("timeout", w.timeout))

	w.done = true

	return true
}
