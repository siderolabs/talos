// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import "time"

// Clock system clock.
type Clock interface {
	Now() time.Time
}

// SystemClock is a real system clock, but the time returned can be made fixed.
type SystemClock struct {
	fixedTime time.Time
}

// NewClock creates new SystemClock.
func NewClock() *SystemClock {
	return &SystemClock{}
}

// NewFixedClock creates new SystemClock with fixed time.
func NewFixedClock(t time.Time) *SystemClock {
	return &SystemClock{
		fixedTime: t,
	}
}

// Now implements Clock.
func (c *SystemClock) Now() time.Time {
	if c.fixedTime.IsZero() {
		return time.Now()
	}

	return c.fixedTime
}
