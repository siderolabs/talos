// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestSchedule(t *testing.T) {
	t.Parallel()

	const interval = 7 * 24 * time.Hour

	now := time.Now()

	t.Run("offset is stable and within interval", func(t *testing.T) {
		t.Parallel()

		offset1 := block.ScheduleOffset("node/volume-a", interval)
		offset2 := block.ScheduleOffset("node/volume-a", interval)

		assert.Equal(t, offset1, offset2)
		assert.GreaterOrEqual(t, offset1, time.Duration(0))
		assert.Less(t, offset1, interval)

		// different seeds (volumes or nodes) are spread across the interval.
		assert.NotEqual(t, offset1, block.ScheduleOffset("node/volume-b", interval))
		assert.NotEqual(t, offset1, block.ScheduleOffset("other-node/volume-a", interval))
	})

	t.Run("next slot is strictly after now", func(t *testing.T) {
		t.Parallel()

		next := block.NextScheduledTime("node/volume-a", interval, now)

		assert.True(t, next.After(now), "next slot must be strictly after now")
		// the previous slot must be at or before now.
		assert.False(t, next.Add(-interval).After(now))
	})

	t.Run("slots form a stable lattice anchored on a known slot", func(t *testing.T) {
		t.Parallel()

		anchor := block.NextScheduledTime("node/volume-a", interval, now)

		// the slot just before the anchor is exactly one interval earlier.
		assert.Equal(t, anchor.Add(-interval), block.ScheduleSlotBefore(anchor, interval, anchor.Add(-time.Nanosecond)))

		// ScheduleSlotBefore returns a slot at or before t.
		before := block.ScheduleSlotBefore(anchor, interval, now)
		assert.False(t, before.After(now))
		assert.True(t, before.Add(interval).After(now))

		// ScheduleSlotAfter returns a slot strictly after t, one interval ahead of ScheduleSlotBefore.
		after := block.ScheduleSlotAfter(anchor, interval, now)
		assert.True(t, after.After(now))
		assert.Equal(t, before.Add(interval), after)

		// anchoring on any slot of the lattice yields the same slots.
		assert.Equal(t, before, block.ScheduleSlotBefore(anchor.Add(5*interval), interval, now))
	})

	t.Run("zero interval is handled", func(t *testing.T) {
		t.Parallel()

		assert.Zero(t, block.ScheduleOffset("node/volume-a", 0))
		assert.True(t, block.NextScheduledTime("node/volume-a", 0, now).IsZero())
		assert.True(t, block.ScheduleSlotBefore(now, 0, now).IsZero())
		assert.True(t, block.ScheduleSlotAfter(now, 0, now).IsZero())
	})
}
