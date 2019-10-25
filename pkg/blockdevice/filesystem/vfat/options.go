// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vfat

// Options is the functional options struct.
type Options struct {
	Label string
}

// WithLabel sets the filesystem label.
func WithLabel(o string) Option {
	return func(args *Options) {
		args.Label = o
	}
}

// Option is the functional option func.
type Option func(*Options)

// NewDefaultOptions initializes a Options struct with default values.
func NewDefaultOptions(setters ...Option) *Options {
	opts := &Options{
		Label: "",
	}

	for _, setter := range setters {
		setter(opts)
	}

	return opts
}
