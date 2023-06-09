// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// RBACEnabled implements config.Features interface.
func (f *FeaturesConfig) RBACEnabled() bool {
	if f.RBAC == nil {
		return false // the current default value
	}

	return *f.RBAC
}

// StableHostnameEnabled implements config.Features interface.
func (f *FeaturesConfig) StableHostnameEnabled() bool {
	return pointer.SafeDeref(f.StableHostname)
}

// KubernetesTalosAPIAccess implements config.Features interface.
func (f *FeaturesConfig) KubernetesTalosAPIAccess() config.KubernetesTalosAPIAccess {
	return f.KubernetesTalosAPIAccessConfig
}

// ApidCheckExtKeyUsageEnabled implements config.Features interface.
func (f *FeaturesConfig) ApidCheckExtKeyUsageEnabled() bool {
	return pointer.SafeDeref(f.ApidCheckExtKeyUsage)
}

// DiskQuotaSupportEnabled implements config.Features interface.
func (f *FeaturesConfig) DiskQuotaSupportEnabled() bool {
	return pointer.SafeDeref(f.DiskQuotaSupport)
}

// APIServerBalancer implements config.Features interface.
func (f *FeaturesConfig) APIServerBalancer() config.APIServerBalancer {
	if f.APIServerBalancerSupport == nil {
		return &APIServerBalancer{}
	}

	return f.APIServerBalancerSupport
}

const defaultAPIBalancerPort = 7445

// Enabled implements config.APIServerBalancer.
func (a *APIServerBalancer) Enabled() bool {
	return pointer.SafeDeref(a.ServerEnabled)
}

// Port implements config.APIServerBalancer.
func (a *APIServerBalancer) Port() int {
	if a.ServerPort == 0 {
		return defaultAPIBalancerPort
	}

	return a.ServerPort
}
