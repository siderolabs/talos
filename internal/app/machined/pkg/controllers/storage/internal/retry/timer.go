// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package retry provides a timer for retrying controller reconciliation with exponential backoff.
package retry

import "time"

const (
	defaultInitialInterval = time.Second
	defaultMaxInterval     = 30 * time.Second
)

// Timer schedules retries with exponential backoff.
type Timer struct {
	timer   *time.Timer
	initial time.Duration
	maximum time.Duration
	next    time.Duration
	armed   bool
}

// NewTimer creates a retry timer with the given initial and maximum intervals.
func NewTimer(initial, maximum time.Duration) *Timer {
	if initial <= 0 {
		initial = defaultInitialInterval
	}

	if maximum < initial {
		maximum = defaultMaxInterval
		if maximum < initial {
			maximum = initial
		}
	}

	timer := time.NewTimer(initial)
	timer.Stop()

	return &Timer{timer: timer, initial: initial, maximum: maximum, next: initial}
}

// C returns the channel on which retry events are delivered.
func (retry *Timer) C() <-chan time.Time {
	return retry.timer.C
}

// Schedule schedules a retry unless one is already pending.
func (retry *Timer) Schedule() {
	if retry.armed {
		return
	}

	retry.timer.Reset(retry.next)
	retry.armed = true

	if retry.next > retry.maximum-retry.next {
		retry.next = retry.maximum
	} else {
		retry.next *= 2
	}
}

// Fired marks the pending retry as delivered.
func (retry *Timer) Fired() {
	retry.armed = false
}

// Reset cancels any pending retry and restores the initial interval.
func (retry *Timer) Reset() {
	retry.timer.Stop()
	retry.armed = false
	retry.next = retry.initial
}

// Stop releases the timer's resources.
func (retry *Timer) Stop() {
	retry.timer.Stop()
}
