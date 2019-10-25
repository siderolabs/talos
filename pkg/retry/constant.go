// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"time"
)

type constantRetryer struct {
	retryer
}

// ConstantTicker represents a ticker with a constant algorithm.
type ConstantTicker struct {
	ticker
}

// Constant initializes and returns a constant Retryer.
func Constant(duration time.Duration, setters ...Option) Retryer {
	opts := NewDefaultOptions(setters...)

	return constantRetryer{
		retryer: retryer{
			duration: duration,
			options:  opts,
		},
	}
}

// NewConstantTicker is a ticker that sends the time on a channel using a
// constant algorithm.
func NewConstantTicker(opts *Options) *ConstantTicker {
	l := &ConstantTicker{
		ticker: ticker{
			C:       make(chan time.Time, 1),
			options: opts,
			s:       make(chan struct{}, 1),
		},
	}

	return l
}

// Retry implements the Retryer interface.
func (c constantRetryer) Retry(f RetryableFunc) error {
	tick := NewConstantTicker(c.options)
	defer tick.Stop()

	return retry(f, c.duration, tick)
}

// Tick implements the Ticker interface.
func (c ConstantTicker) Tick() time.Duration {
	return c.options.Units + c.Jitter()
}
