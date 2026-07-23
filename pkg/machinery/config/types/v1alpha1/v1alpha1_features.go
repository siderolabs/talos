// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// DiskQuotaSupportEnabled implements config.Features interface.
func (f *FeaturesConfig) DiskQuotaSupportEnabled() bool {
	return pointer.SafeDeref(f.DiskQuotaSupport)
}

// NodeAddressSortAlgorithm implements config.Features interface.
func (f *FeaturesConfig) NodeAddressSortAlgorithm() nethelpers.AddressSortAlgorithm {
	if f.FeatureNodeAddressSortAlgorithm == "" {
		return nethelpers.AddressSortAlgorithmV1
	}

	res, err := nethelpers.AddressSortAlgorithmString(f.FeatureNodeAddressSortAlgorithm)
	if err != nil {
		return nethelpers.AddressSortAlgorithmV1
	}

	return res
}

const defaultKubePrismPort = 7445

// Enabled is  a legacy method.
//
// New implementation returns nil interface if the feature is not enabled.
func (a *KubePrism) Enabled() bool {
	return pointer.SafeDeref(a.ServerEnabled)
}

// Port implements [config.K8sKubePrismConfig].
func (a *KubePrism) Port() int {
	if a.ServerPort == 0 {
		return defaultKubePrismPort
	}

	return a.ServerPort
}

// TLSServerName implements [config.K8sKubePrismConfig].
func (a *KubePrism) TLSServerName() string {
	return ""
}

// K8sKubePrismConfigSignal implements [config.K8sKubePrismConfig] interface.
func (a *KubePrism) K8sKubePrismConfigSignal() {}

// HostDNSEnabled implements config.NetworkHostDNSConfig interface.
func (h *HostDNSConfig) HostDNSEnabled() bool {
	return pointer.SafeDeref(h.HostDNSConfigEnabled)
}

// ForwardKubeDNSToHost implements config.NetworkHostDNSConfig interface.
func (h *HostDNSConfig) ForwardKubeDNSToHost() bool {
	return pointer.SafeDeref(h.HostDNSForwardKubeDNSToHost)
}

// ResolveMemberNames implements config.NetworkHostDNSConfig interface.
func (h *HostDNSConfig) ResolveMemberNames() bool {
	return pointer.SafeDeref(h.HostDNSResolveMemberNames)
}

// LocalEnabled implements config.ImageCache.
func (i *ImageCacheConfig) LocalEnabled() bool {
	return pointer.SafeDeref(i.CacheLocalEnabled)
}
