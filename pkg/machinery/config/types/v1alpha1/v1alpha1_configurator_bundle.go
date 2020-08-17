// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// ConfigBundle defines the group of v1alpha1 config files.
//docgen: nodoc
type ConfigBundle struct {
	InitCfg         *Config
	ControlPlaneCfg *Config
	JoinCfg         *Config
	TalosCfg        *clientconfig.Config
}

// Init implements the ConfiguratorBundle interface.
func (c *ConfigBundle) Init() config.Provider {
	return c.InitCfg
}

// ControlPlane implements the ConfiguratorBundle interface.
func (c *ConfigBundle) ControlPlane() config.Provider {
	return c.ControlPlaneCfg
}

// Join implements the ConfiguratorBundle interface.
func (c *ConfigBundle) Join() config.Provider {
	return c.JoinCfg
}

// TalosConfig implements the ConfiguratorBundle interface.
func (c *ConfigBundle) TalosConfig() *clientconfig.Config {
	return c.TalosCfg
}
