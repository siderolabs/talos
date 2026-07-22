// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"hash/fnv"
	"time"
)

// Slot helpers for periodic per-volume maintenance operations (trim, scrub).
//
// Slots form a stable lattice anchored at the Unix epoch: offset, offset+interval,
// offset+2*interval, ... where the offset is derived from a seed, so the schedule is
// stable across reboots while being spread out across volumes and nodes.

// ScheduleOffset returns the stable offset within the interval for a seed.
//
// The offset is derived by hashing the seed (e.g. node ID + volume ID), so it stays
// constant for a given seed and interval, spreading the operations across the interval -
// both across volumes on a node and across nodes in a cluster.
func ScheduleOffset(seed string, interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}

	h := fnv.New64a()
	h.Write([]byte(seed)) //nolint:errcheck // hash.Hash.Write never returns an error

	return time.Duration(h.Sum64() % uint64(interval))
}

// NextScheduledTime returns the earliest slot strictly after t for the seed.
func NextScheduledTime(seed string, interval time.Duration, t time.Time) time.Time {
	if interval <= 0 {
		return time.Time{}
	}

	anchor := time.Unix(0, int64(ScheduleOffset(seed, interval)))

	return ScheduleSlotAfter(anchor, interval, t)
}

// ScheduleSlotBefore returns the most recent slot at or before t on the lattice
// anchored at the given slot (anchor + k*interval for integer k).
//
// It only needs a single known slot (anchor) and the interval, so it does not depend
// on the seed used to compute the schedule.
func ScheduleSlotBefore(anchor time.Time, interval time.Duration, t time.Time) time.Time {
	if interval <= 0 {
		return time.Time{}
	}

	step := int64(interval)
	diff := t.UnixNano() - anchor.UnixNano()

	// number of full intervals between the anchor and t (floored).
	k := diff / step
	if diff%step < 0 {
		k--
	}

	return anchor.Add(time.Duration(k) * interval)
}

// ScheduleSlotAfter returns the earliest slot strictly after t on the lattice
// anchored at the given slot.
func ScheduleSlotAfter(anchor time.Time, interval time.Duration, t time.Time) time.Time {
	if interval <= 0 {
		return time.Time{}
	}

	slot := ScheduleSlotBefore(anchor, interval, t)
	if !slot.After(t) {
		slot = slot.Add(interval)
	}

	return slot
}
