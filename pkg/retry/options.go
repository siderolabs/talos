// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import "time"

// Options is the functional options struct.
type Options struct {
	Units  time.Duration
	Jitter time.Duration
}

// Option is the functional option func.
type Option func(*Options)

// WithUnits is a functional option for setting the units of the ticker.
func WithUnits(o time.Duration) Option {
	return func(args *Options) {
		args.Units = o
	}
}

// WithJitter is a functional option for setting the jitter flag.
func WithJitter(o time.Duration) Option {
	return func(args *Options) {
		args.Jitter = o
	}
}

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Units:  time.Second,
		Jitter: time.Duration(0),
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
