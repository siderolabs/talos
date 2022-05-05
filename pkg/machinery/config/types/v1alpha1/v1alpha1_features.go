// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import "github.com/talos-systems/talos/pkg/machinery/config"

// RBACEnabled implements config.Features interface.
func (f *FeaturesConfig) RBACEnabled() bool {
	if f.RBAC == nil {
		return false // the current default value
	}

	return *f.RBAC
}

// KubernetesTalosAPIAccess implements config.Features interface.
func (f *FeaturesConfig) KubernetesTalosAPIAccess() config.KubernetesTalosAPIAccess {
	return f.KubernetesTalosAPIAccessConfig
}
