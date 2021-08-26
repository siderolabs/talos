// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Enabled implements the config.ClusterDiscovery interface.
func (c ClusterDiscoveryConfig) Enabled() bool {
	return c.DiscoveryEnabled
}

// Registries implements the config.ClusterDiscovery interface.
func (c ClusterDiscoveryConfig) Registries() config.DiscoveryRegistries {
	return c.DiscoveryRegistries
}

// Kubernetes implements the config.DiscoveryRegistries interface.
func (c DiscoveryRegistriesConfig) Kubernetes() config.KubernetesRegistry {
	return c.RegistryKubernetes
}

// Service implements the config.DiscoveryRegistries interface.
func (c DiscoveryRegistriesConfig) Service() config.ServiceRegistry {
	return c.RegistryService
}

// Enabled implements the config.KubernetesRegistry interface.
func (c RegistryKubernetesConfig) Enabled() bool {
	return !c.RegistryDisabled
}

// Enabled implements the config.ServiceRegistry interface.
func (c RegistryServiceConfig) Enabled() bool {
	return !c.RegistryDisabled
}

// Endpoint implements the config.ServiceRegistry interface.
func (c RegistryServiceConfig) Endpoint() string {
	if c.RegistryEndpoint == "" {
		return constants.DefaultDiscoveryServiceEndpoint
	}

	return c.RegistryEndpoint
}
