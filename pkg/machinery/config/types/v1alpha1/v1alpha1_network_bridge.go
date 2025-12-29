// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"net/netip"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// This file contains methods which bridge v1alpha1 (legacy) config types to new-style config interfaces for networking.

// NetworkStaticHostConfig implements config.NetworkStaticHostConfig interface.
func (c *Config) NetworkStaticHostConfig() []config.NetworkStaticHostConfig {
	if c == nil || c.MachineConfig == nil || c.MachineConfig.MachineNetwork == nil {
		return nil
	}

	return c.MachineConfig.MachineNetwork.ExtraHosts()
}

// Hostname implements config.NetworkHostnameConfig interface.
func (c *Config) Hostname() string {
	if c.MachineConfig == nil || c.MachineConfig.MachineNetwork == nil {
		return ""
	}

	return c.MachineConfig.MachineNetwork.NetworkHostname
}

// AutoHostname implements config.NetworkHostnameConfig interface.
func (c *Config) AutoHostname() nethelpers.AutoHostnameKind {
	if c.MachineConfig == nil || c.MachineConfig.MachineFeatures == nil {
		// legacy mode
		return nethelpers.AutoHostnameKindAddr
	}

	if pointer.SafeDeref(c.MachineConfig.MachineFeatures.StableHostname) {
		return nethelpers.AutoHostnameKindStable
	}

	return nethelpers.AutoHostnameKindAddr
}

// Resolvers implements config.NetworkResolverConfig interface.
func (c *Config) Resolvers() []netip.Addr {
	if c.MachineConfig == nil || c.MachineConfig.MachineNetwork == nil {
		return nil
	}

	var result []netip.Addr

	for _, r := range c.MachineConfig.MachineNetwork.NameServers {
		if addr, err := netip.ParseAddr(r); err == nil {
			result = append(result, addr)
		}
	}

	return result
}

// SearchDomains implements config.NetworkResolverConfig interface.
func (c *Config) SearchDomains() []string {
	if c.MachineConfig == nil || c.MachineConfig.MachineNetwork == nil {
		return nil
	}

	return c.MachineConfig.MachineNetwork.Searches
}

// DisableSearchDomain implements config.NetworkResolverConfig interface.
func (c *Config) DisableSearchDomain() bool {
	if c.MachineConfig == nil || c.MachineConfig.MachineNetwork == nil {
		return false
	}

	return pointer.SafeDeref(c.MachineConfig.MachineNetwork.NetworkDisableSearchDomain)
}

// NetworkTimeSyncConfig implements config.NetworkTimeSyncConfig interface.
func (c *Config) NetworkTimeSyncConfig() config.NetworkTimeSyncConfig {
	if c.MachineConfig == nil || c.MachineConfig.MachineTime == nil {
		return nil
	}

	return c.MachineConfig.MachineTime
}

// NetworkKubeSpanConfig implements the config.NetworkKubeSpanConfig interface.
func (c *Config) NetworkKubeSpanConfig() config.NetworkKubeSpanConfig {
	if c.MachineConfig == nil || c.MachineConfig.MachineNetwork == nil || c.MachineConfig.MachineNetwork.NetworkKubeSpan == nil {
		return nil
	}

	return c.MachineConfig.MachineNetwork.NetworkKubeSpan
}
