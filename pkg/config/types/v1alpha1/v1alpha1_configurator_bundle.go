// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/cmd/talosctl/pkg/client/config"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// ConfigBundle defines the group of v1alpha1 config files.
//docgen: nodoc
type ConfigBundle struct {
	InitCfg         *Config
	ControlPlaneCfg *Config
	JoinCfg         *Config
	HostCfgs        map[string]*Config
	TalosCfg        *config.Config
}

// Init implements the ConfiguratorBundle interface.
func (c *ConfigBundle) Init() runtime.Configurator {
	return c.InitCfg
}

// ControlPlane implements the ConfiguratorBundle interface.
func (c *ConfigBundle) ControlPlane() runtime.Configurator {
	return c.ControlPlaneCfg
}

// Join implements the ConfiguratorBundle interface.
func (c *ConfigBundle) Join() runtime.Configurator {
	return c.JoinCfg
}

// TalosConfig implements the ConfiguratorBundle interface.
func (c *ConfigBundle) TalosConfig() *config.Config {
	return c.TalosCfg
}

// Hosts returns host-specific configurations.
func (c *ConfigBundle) Hosts() map[string]*Config {
	return c.HostCfgs
}
