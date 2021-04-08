// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// ValidationOptions additional validation parameters for the config Validate method.
type ValidationOptions struct {
	// Local should disable part of the validation flow which won't work on the host machine.
	Local bool
	// Strict mode returns warnings as errors.
	Strict bool
}

// ValidationOption represents an additional validation parameter for the config Validate method.
type ValidationOption func(opts *ValidationOptions)

// NewValidationOptions creates new validation options.
func NewValidationOptions(options ...ValidationOption) *ValidationOptions {
	opts := &ValidationOptions{}
	for _, f := range options {
		f(opts)
	}

	return opts
}

// WithLocal enables local flag.
func WithLocal() ValidationOption {
	return func(opts *ValidationOptions) {
		opts.Local = true
	}
}

// WithStrict enables strict flag.
func WithStrict() ValidationOption {
	return func(opts *ValidationOptions) {
		opts.Strict = true
	}
}
