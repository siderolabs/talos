// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"time"
)

type linearRetryer struct {
	retryer
}

// LinearTicker represents a ticker with a linear algorithm.
type LinearTicker struct {
	ticker

	c int
}

// Linear initializes and returns a linear Retryer.
func Linear(duration time.Duration, setters ...Option) Retryer {
	opts := NewDefaultOptions(setters...)

	return linearRetryer{
		retryer: retryer{
			duration: duration,
			options:  opts,
		},
	}
}

// NewLinearTicker is a ticker that sends the time on a channel using a
// linear algorithm.
func NewLinearTicker(opts *Options) *LinearTicker {
	l := &LinearTicker{
		ticker: ticker{
			C:       make(chan time.Time, 1),
			options: opts,
			s:       make(chan struct{}, 1),
		},
		c: 1,
	}

	return l
}

// Retry implements the Retryer interface.
func (l linearRetryer) Retry(f RetryableFunc) error {
	tick := NewLinearTicker(l.options)
	defer tick.Stop()

	return retry(f, l.duration, tick)
}

// Tick implements the Ticker interface.
func (l *LinearTicker) Tick() time.Duration {
	d := time.Duration(l.c)*l.options.Units + l.Jitter()
	l.c++

	return d
}
