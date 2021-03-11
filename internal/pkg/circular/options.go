// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package circular

import "fmt"

// Options defines settings for Buffer.
type Options struct {
	InitialCapacity int
	MaxCapacity     int
	SafetyGap       int
}

// defaultOptions returns default initial values.
func defaultOptions() Options {
	return Options{
		InitialCapacity: 16384,
		MaxCapacity:     1048576,
		SafetyGap:       1024,
	}
}

// OptionFunc allows setting Buffer options.
type OptionFunc func(*Options) error

// WithInitialCapacity sets initial buffer capacity.
func WithInitialCapacity(capacity int) OptionFunc {
	return func(opt *Options) error {
		if capacity <= 0 {
			return fmt.Errorf("initial capacity should be positive: %d", capacity)
		}

		opt.InitialCapacity = capacity

		return nil
	}
}

// WithMaxCapacity sets maximum buffer capacity.
func WithMaxCapacity(capacity int) OptionFunc {
	return func(opt *Options) error {
		if capacity <= 0 {
			return fmt.Errorf("max capacity should be positive: %d", capacity)
		}

		opt.MaxCapacity = capacity

		return nil
	}
}

// WithSafetyGap sets safety gap between readers and writers to avoid buffer overrun for the reader.
//
// Reader initial position is set to be as far as possible in the buffer history, but next concurrent write
// might overwrite read position, and safety gap helps to prevent it. With safety gap, maximum available
// bytes to read are: MaxCapacity-SafetyGap.
func WithSafetyGap(gap int) OptionFunc {
	return func(opt *Options) error {
		if gap <= 0 {
			return fmt.Errorf("safety gap should be positive: %q", gap)
		}

		opt.SafetyGap = gap

		return nil
	}
}
