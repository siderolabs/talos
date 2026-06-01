// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package validation provides validation options for the config Validate method.
package validation

// Options additional validation parameters for the config Validate method.
type Options struct {
	// Local should disable part of the validation flow which won't work on the host machine.
	Local bool
	// Strict mode returns warnings as errors.
	Strict bool
}

// Option represents an additional validation parameter for the config Validate method.
type Option func(opts *Options)

// NewOptions creates new validation options.
func NewOptions(options ...Option) *Options {
	opts := &Options{}
	for _, f := range options {
		f(opts)
	}

	return opts
}

// WithLocal enables local flag.
func WithLocal() Option {
	return func(opts *Options) {
		opts.Local = true
	}
}

// WithStrict enables strict flag.
func WithStrict() Option {
	return func(opts *Options) {
		opts.Strict = true
	}
}
