// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package makefs provides function to format and grow filesystems.
package makefs

// Option to control makefs settings.
type Option func(*Options)

// Options for makefs.
type Options struct {
	Label string
	Force bool
}

// WithLabel sets the label for the filesystem to be created.
func WithLabel(label string) Option {
	return func(o *Options) {
		o.Label = label
	}
}

// WithForce forces creation of a filesystem even if one already exists.
func WithForce(force bool) Option {
	return func(o *Options) {
		o.Force = force
	}
}

// NewDefaultOptions builds options with specified setters applied.
func NewDefaultOptions(setters ...Option) Options {
	var opt Options

	for _, o := range setters {
		o(&opt)
	}

	return opt
}
