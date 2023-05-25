// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bundle

import "github.com/siderolabs/talos/pkg/machinery/config/bundle"

// ConfigBundle defines the group of v1alpha1 config files.
// docgen: nodoc
// +k8s:deepcopy-gen=false
type ConfigBundle = bundle.Bundle

// NewConfigBundle returns a new bundle.
//
// Deprecated: use bundle.NewBundle instead.
func NewConfigBundle(opts ...Option) (*ConfigBundle, error) {
	return bundle.NewBundle(opts...)
}
