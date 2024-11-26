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

// HostDNS implements config.Features interface.
func (f *FeaturesConfig) HostDNS() config.HostDNS {
	if f.HostDNSSupport == nil {
		return &HostDNSConfig{}
	}

	return f.HostDNSSupport
}

// KubePrism implements config.Features interface.
func (f *FeaturesConfig) KubePrism() config.KubePrism {
	if f.KubePrismSupport == nil {
		return &KubePrism{}
	}

	return f.KubePrismSupport
}

// LongPrefixPreferenceEnabled implements config.Features interface.
func (f *FeaturesConfig) LongPrefixPreferenceEnabled() bool {
	return pointer.SafeDeref(f.LongPrefixPreference)
}

const defaultKubePrismPort = 7445

// Enabled implements [config.KubePrism].
func (a *KubePrism) Enabled() bool {
	return pointer.SafeDeref(a.ServerEnabled)
}

// Port implements [config.KubePrism].
func (a *KubePrism) Port() int {
	if a.ServerPort == 0 {
		return defaultKubePrismPort
	}

	return a.ServerPort
}

// Enabled implements config.HostDNS.
func (h *HostDNSConfig) Enabled() bool {
	return pointer.SafeDeref(h.HostDNSEnabled)
}

// ForwardKubeDNSToHost implements config.HostDNS.
func (h *HostDNSConfig) ForwardKubeDNSToHost() bool {
	return pointer.SafeDeref(h.HostDNSForwardKubeDNSToHost)
}

// ResolveMemberNames implements config.HostDNS.
func (h *HostDNSConfig) ResolveMemberNames() bool {
	return pointer.SafeDeref(h.HostDNSResolveMemberNames)
}
