// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"

// Option represents functional option.
type Option func(o *Options) error

// WithNodeTypes sets the node types for a check.
func WithNodeTypes(t ...machine.Type) Option {
	return func(o *Options) error {
		o.Types = t

		return nil
	}
}

// Options describes ClusterCheck parameters.
type Options struct {
	Types []machine.Type
}

// DefaultOptions returns the default options.
func DefaultOptions() *Options {
	return &Options{}
}
