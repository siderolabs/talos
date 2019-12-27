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

// WithForceInitNodeAsEndpoint uses direct IP of init node as endpoint instead of (default)
// mode.
func WithForceInitNodeAsEndpoint() Option {
	return func(o *Options) error {
		o.ForceInitNodeAsEndpoint = true

		return nil
	}
}

// WithEndpoint specifies endpoint to use when acessing Talos cluster.
func WithEndpoint(endpoint string) Option {
	return func(o *Options) error {
		o.ForceEndpoint = endpoint

		return nil
	}
}

// Options describes Provisioner parameters.
type Options struct {
	LogWriter               io.Writer
	ForceInitNodeAsEndpoint bool
	ForceEndpoint           string
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{
		LogWriter: os.Stderr,
	}
}
