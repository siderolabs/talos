// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

// Option controls generate options specific to input generation.
type Option func(o *Options) error

// WithImagePull disables pulling the installer image during an install
func WithImagePull(shouldPull bool) Option {
	return func(o *Options) error {
		o.ImagePull = shouldPull

		return nil
	}
}

// Options describes generate parameters.
type Options struct {
	ImagePull bool
}

// DefaultInstallOptions returns default options.
func DefaultInstallOptions() Options {
	return Options{
		ImagePull: true,
	}
}
