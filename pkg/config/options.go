// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"

// BundleOption controls config options specific to config bundle generation.
type BundleOption func(o *BundleOptions) error

// InputOptions holds necessary params for generating an input
type InputOptions struct {
	ClusterName               string
	Endpoint                  string
	KubeVersion               string
	AdditionalSubjectAltNames []string
	InstallDisk               string
	InstallImage              string
	GenOptions                []generate.GenOption
}

// BundleOptions describes generate parameters.
type BundleOptions struct {
	ExistingConfigs string // path to existing config files
	InputOptions    *InputOptions
}

// DefaultBundleOptions returns default options.
func DefaultBundleOptions() BundleOptions {
	return BundleOptions{}
}

// WithExistingConfigs sets the path to existing config files
func WithExistingConfigs(configPath string) BundleOption {
	return func(o *BundleOptions) error {
		o.ExistingConfigs = configPath
		return nil
	}
}

// WithInputOptions allows passing in of various params for net-new input generation
func WithInputOptions(inputOpts *InputOptions) BundleOption {
	return func(o *BundleOptions) error {
		o.InputOptions = inputOpts
		return nil
	}
}
