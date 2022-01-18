// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerd provides support for containerd CRI plugin
package containerd

import (
	"bytes"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// GenerateCRIConfig returns a part of CRI config for registry auth.
//
// Once containerd supports different way of supplying auth info, this should be updated.
func GenerateCRIConfig(r config.Registries) ([]byte, error) {
	var ctrdCfg Config

	ctrdCfg.Plugins.CRI.Registry.ConfigPath = filepath.Join(constants.CRIConfdPath, "hosts")
	ctrdCfg.Plugins.CRI.Registry.Configs = make(map[string]RegistryConfig)

	for registryHost, hostConfig := range r.Config() {
		if hostConfig.Auth() != nil {
			cfg := RegistryConfig{}
			cfg.Auth = &AuthConfig{
				Username:      hostConfig.Auth().Username(),
				Password:      hostConfig.Auth().Password(),
				Auth:          hostConfig.Auth().Auth(),
				IdentityToken: hostConfig.Auth().IdentityToken(),
			}
			ctrdCfg.Plugins.CRI.Registry.Configs[registryHost] = cfg
		}
	}

	var buf bytes.Buffer

	if err := toml.NewEncoder(&buf).Encode(&ctrdCfg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
