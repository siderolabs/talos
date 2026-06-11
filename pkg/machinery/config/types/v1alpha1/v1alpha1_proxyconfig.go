// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Enabled implements the config.Proxy interface.
func (p *ProxyConfig) Enabled() bool {
	return !pointer.SafeDeref(p.Disabled)
}

// Image implements the config.Proxy interface.
func (p *ProxyConfig) Image() string {
	image := p.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubeProxyImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// Mode implements the config.Proxy interface.
func (p *ProxyConfig) Mode() string {
	return p.ModeConfig
}

// ExtraArgs implements the config.Proxy interface.
func (p *ProxyConfig) ExtraArgs() map[string][]string {
	return p.ExtraArgsConfig.ToMap()
}

// Resources implements the config.Proxy interface.
func (p *ProxyConfig) Resources() config.Resources {
	return &ResourcesConfig{}
}

// Config implements the config.Proxy interface.
func (p *ProxyConfig) Config() map[string]any {
	return nil
}

// UseConfigFile implements the config.Proxy interface.
func (p *ProxyConfig) UseConfigFile() bool {
	return false
}

// K8sProxyConfigSignal implements the config.K8sProxyConfig interface.
func (p *ProxyConfig) K8sProxyConfigSignal() {}
