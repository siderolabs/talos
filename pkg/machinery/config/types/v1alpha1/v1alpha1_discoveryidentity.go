// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// legacyDiscoveryIdentityConfig adapts the v1alpha1 cluster identity config to the config.DiscoveryIdentityConfig interface.
type legacyDiscoveryIdentityConfig struct {
	id     string
	secret string
}

// ClusterID implements config.DiscoveryIdentityConfig interface.
func (c legacyDiscoveryIdentityConfig) ClusterID() string {
	return c.id
}

// ClusterSecret implements config.DiscoveryIdentityConfig interface.
func (c legacyDiscoveryIdentityConfig) ClusterSecret() string {
	return c.secret
}

// DiscoveryIdentityConfig returns the cluster identity config derived from the legacy v1alpha1 cluster config.
func (c *Config) DiscoveryIdentityConfig() config.DiscoveryIdentityConfig {
	if c.ClusterConfig == nil ||
		(c.ClusterConfig.ClusterID == "" && c.ClusterConfig.ClusterSecret == "") { //nolint:staticcheck // legacy configuration
		return nil
	}

	return legacyDiscoveryIdentityConfig{
		id:     c.ClusterConfig.ClusterID,     //nolint:staticcheck // legacy configuration
		secret: c.ClusterConfig.ClusterSecret, //nolint:staticcheck // legacy configuration
	}
}
