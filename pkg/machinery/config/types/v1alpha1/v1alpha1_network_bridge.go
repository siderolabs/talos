// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
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
