// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"net/url"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// legacyDiscoveryServiceConfig adapts the v1alpha1 cluster discovery config to the config.DiscoveryServiceConfig interface.
type legacyDiscoveryServiceConfig struct {
	endpoint *url.URL
}

// Name implements config.DiscoveryServiceConfig interface.
func (legacyDiscoveryServiceConfig) Name() string {
	return "legacy"
}

// Endpoint implements config.DiscoveryServiceConfig interface.
func (c legacyDiscoveryServiceConfig) Endpoint() *url.URL {
	return c.endpoint
}

// DiscoveryServiceConfigs returns the discovery service configs derived from the legacy v1alpha1 cluster discovery config.
func (c *Config) DiscoveryServiceConfigs() []config.DiscoveryServiceConfig {
	if c.ClusterConfig == nil || c.ClusterConfig.ClusterDiscoveryConfig == nil ||
		!pointer.SafeDeref(c.ClusterConfig.ClusterDiscoveryConfig.DiscoveryEnabled) ||
		!c.ClusterConfig.ClusterDiscoveryConfig.DiscoveryRegistries.RegistryService.Enabled() {
		return nil
	}

	endpoint, err := url.Parse(c.ClusterConfig.ClusterDiscoveryConfig.DiscoveryRegistries.RegistryService.Endpoint())
	if err != nil {
		return nil
	}

	return []config.DiscoveryServiceConfig{legacyDiscoveryServiceConfig{endpoint: endpoint}}
}

// Enabled implements the config.ClusterDiscovery interface.
func (c *ClusterDiscoveryConfig) Enabled() bool {
	return pointer.SafeDeref(c.DiscoveryEnabled)
}

// Registries implements the config.ClusterDiscovery interface.
func (c *ClusterDiscoveryConfig) Registries() config.DiscoveryRegistries {
	return c.DiscoveryRegistries
}

// Kubernetes implements the config.DiscoveryRegistries interface.
func (c DiscoveryRegistriesConfig) Kubernetes() config.KubernetesRegistry {
	return c.RegistryKubernetes
}

// Service implements the config.DiscoveryRegistries interface.
func (c DiscoveryRegistriesConfig) Service() RegistryServiceConfig {
	return c.RegistryService
}

// Enabled implements the config.KubernetesRegistry interface.
func (c RegistryKubernetesConfig) Enabled() bool {
	return !pointer.SafeDeref(c.RegistryDisabled)
}

// Enabled implements the config.ServiceRegistry interface.
func (c RegistryServiceConfig) Enabled() bool {
	return !pointer.SafeDeref(c.RegistryDisabled)
}

// Endpoint implements the config.ServiceRegistry interface.
func (c RegistryServiceConfig) Endpoint() string {
	if c.RegistryEndpoint == "" {
		return constants.DefaultDiscoveryServiceEndpoint
	}

	return c.RegistryEndpoint
}
