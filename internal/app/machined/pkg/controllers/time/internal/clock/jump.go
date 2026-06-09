// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package clock provides clock-related privimites, e.g. jump detection.
package clock

import (
	"context"
	"time"

	"github.com/siderolabs/gen/channel"
)

// DefaultJumpDetectionInterval is the default interval for wall-clock jump detection.
const DefaultJumpDetectionInterval = time.Minute

// WallClockJumpDetector detects wall-clock jumps by comparing elapsed time of wall clock and monotonic clock.
type WallClockJumpDetector struct {
	lastWallClock      time.Time
	lastMonotonicClock time.Time
	interval           time.Duration
	threshold          time.Duration
}

// NewWallClockJumpDetector creates a new WallClockJumpDetector.
func NewWallClockJumpDetector(interval, threshold time.Duration) *WallClockJumpDetector {
	now := time.Now()

	return &WallClockJumpDetector{
		lastWallClock:      wallClockOnly(now),
		lastMonotonicClock: now,
		interval:           interval,
		threshold:          threshold,
	}
}

// Run starts the jump detector loop.
func (d *WallClockJumpDetector) Run(ctx context.Context) <-chan struct{} {
	jumpCh := make(chan struct{}, 1)

	go func() {
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			now := time.Now()

			wallClockElapsed := wallClockOnly(now).Sub(d.lastWallClock)
			monotonicElapsed := now.Sub(d.lastMonotonicClock)

			if (wallClockElapsed - monotonicElapsed).Abs() > d.threshold {
				if !channel.SendWithContext(ctx, jumpCh, struct{}{}) {
					return
				}

				// remember new clock readings
				d.lastWallClock = wallClockOnly(now)
				d.lastMonotonicClock = now
			}
		}
	}()

	return jumpCh
}

func wallClockOnly(now time.Time) time.Time {
	// Round(0) strips the monotonic clock reading so Sub compares wall-clock time.
	return now.Round(0)
}
