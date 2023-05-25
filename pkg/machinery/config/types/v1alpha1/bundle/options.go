// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bundle

import (
	jsonpatch "github.com/evanphx/json-patch"

	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
)

// Option controls config options specific to config bundle generation.
//
// Deprecated: user bundle.Option instead.
type Option = bundle.Option

// InputOptions holds necessary params for generating an input.
//
// Deprecated: user bundle.InputOptions instead.
type InputOptions = bundle.InputOptions

// Options describes generate parameters.
//
// Deprecated: user bundle.Options instead.
type Options = bundle.Options

// DefaultOptions returns default options.
//
// Deprecated: user bundle.DefaultOptions instead.
func DefaultOptions() Options {
	return bundle.DefaultOptions()
}

// WithExistingConfigs sets the path to existing config files.
//
// Deprecated: use bundle.WithExistingConfigs instead.
func WithExistingConfigs(configPath string) Option {
	return bundle.WithExistingConfigs(configPath)
}

// WithInputOptions allows passing in of various params for net-new input generation.
//
// Deprecated: use bundle.WithInputOptions instead.
func WithInputOptions(inputOpts *InputOptions) Option {
	return bundle.WithInputOptions(inputOpts)
}

// WithVerbose allows setting verbose logging.
//
// Deprecated: use bundle.WithVerbose instead.
func WithVerbose(verbose bool) Option {
	return bundle.WithVerbose(verbose)
}

// WithJSONPatch allows patching every config in a bundle with a patch.
//
// Deprecated: use WithPatch instead.
func WithJSONPatch(patch jsonpatch.Patch) Option {
	return WithPatch([]configpatcher.Patch{patch})
}

// WithPatch allows patching every config in a bundle with a patch.
//
// Deprecated: use bundle.WithPatch instead.
func WithPatch(patch []configpatcher.Patch) Option {
	return bundle.WithPatch(patch)
}

// WithJSONPatchControlPlane allows patching init and controlplane config in a bundle with a patch.
//
// Deprecated: use WithPatchControlPlane instead.
func WithJSONPatchControlPlane(patch jsonpatch.Patch) Option {
	return WithPatchControlPlane([]configpatcher.Patch{patch})
}

// WithPatchControlPlane allows patching init and controlplane config in a bundle with a patch.
//
// Deprecated: use bundle.WithPatchControlPlane instead.
func WithPatchControlPlane(patch []configpatcher.Patch) Option {
	return bundle.WithPatchControlPlane(patch)
}

// WithJSONPatchWorker allows patching worker config in a bundle with a patch.
//
// Deprecated: use WithPatchWorker instead.
func WithJSONPatchWorker(patch jsonpatch.Patch) Option {
	return WithPatchWorker([]configpatcher.Patch{patch})
}

// WithPatchWorker allows patching worker config in a bundle with a patch.
//
// Deprecated: use bundle.WithPatchWorker instead.
func WithPatchWorker(patch []configpatcher.Patch) Option {
	return bundle.WithPatchWorker(patch)
}
