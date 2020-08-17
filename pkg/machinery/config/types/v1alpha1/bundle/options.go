// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bundle

import "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"

// Option controls config options specific to config bundle generation.
type Option func(o *Options) error

// InputOptions holds necessary params for generating an input.
type InputOptions struct {
	ClusterName string
	Endpoint    string
	KubeVersion string
	GenOptions  []generate.GenOption
}

// Options describes generate parameters.
type Options struct {
	ExistingConfigs string // path to existing config files
	InputOptions    *InputOptions
}

// DefaultOptions returns default options.
func DefaultOptions() Options {
	return Options{}
}

// WithExistingConfigs sets the path to existing config files.
func WithExistingConfigs(configPath string) Option {
	return func(o *Options) error {
		o.ExistingConfigs = configPath
		return nil
	}
}

// WithInputOptions allows passing in of various params for net-new input generation.
func WithInputOptions(inputOpts *InputOptions) Option {
	return func(o *Options) error {
		o.InputOptions = inputOpts
		return nil
	}
}
