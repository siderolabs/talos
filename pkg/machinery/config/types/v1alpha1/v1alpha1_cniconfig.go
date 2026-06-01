// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Name implements the config.CNI interface.
func (c *CNIConfig) Name() string {
	return c.CNIName
}

// URLs implements the config.CNI interface.
func (c *CNIConfig) URLs() []string {
	return c.CNIUrls
}

// Flannel implements the config.CNI interface.
func (c *CNIConfig) Flannel() config.FlannelCNI {
	return c.CNIFlannel
}

// ExtraArgs implements the config.FlannelCNI interface.
func (c *FlannelCNIConfig) ExtraArgs() []string {
	if c == nil {
		return nil
	}

	return c.FlanneldExtraArgs
}

// KubeNetworkPoliciesEnabled implements the config.FlannelCNI interface.
func (c *FlannelCNIConfig) KubeNetworkPoliciesEnabled() bool {
	if c == nil {
		return false
	}

	return pointer.SafeDeref(c.FlannelKubeNetworkPoliciesEnabled)
}
