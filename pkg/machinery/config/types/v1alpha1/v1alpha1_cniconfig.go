// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
func (c *CNIConfig) Flannel() config.K8sFlannelCNIConfig {
	if c.CNIFlannel == nil {
		return &FlannelCNIConfig{}
	}

	return c.CNIFlannel
}

// BackendType implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) BackendType() string {
	return constants.FlannelDefaultBackend
}

// BackendPort implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) BackendPort() optional.Optional[uint16] {
	return optional.Some[uint16](constants.FlannelDefaultBackendPort)
}

// BackendMTU implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) BackendMTU() optional.Optional[uint32] {
	return optional.None[uint32]()
}

// BackendExtraConfig implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) BackendExtraConfig() map[string]any {
	return nil
}

// Resources implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) Resources() config.Resources {
	return &ResourcesConfig{}
}

// ExtraArgs implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) ExtraArgs() []string {
	return c.FlanneldExtraArgs
}

// KubeNetworkPoliciesEnabled implements the config.K8sFlannelCNIConfig interface.
func (c *FlannelCNIConfig) KubeNetworkPoliciesEnabled() bool {
	return pointer.SafeDeref(c.FlannelKubeNetworkPoliciesEnabled)
}
