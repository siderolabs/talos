// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"io"
	"os"
)

// Option controls Provisioner.
type Option func(o *Options) error

// WithLogWriter sets logging destination.
func WithLogWriter(w io.Writer) Option {
	return func(o *Options) error {
		o.LogWriter = w

		return nil
	}
}

// Options describes Provisioner parameters.
type Options struct {
	LogWriter io.Writer
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{
		LogWriter: os.Stderr,
	}
}
