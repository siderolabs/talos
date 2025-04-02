// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerd provides support for containerd CRI plugin
package containerd

import (
	"bytes"
	"maps"
	"path/filepath"
	"slices"

	"github.com/containerd/containerd/v2/core/remotes/docker"
	"github.com/pelletier/go-toml/v2"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// GenerateCRIConfig returns a part of CRI config for registry auth.
//
// Once containerd supports different way of supplying auth info, this should be updated.
func GenerateCRIConfig(r config.Registries) ([]byte, error) {
	var ctrdCfg Config

	ctrdCfg.Plugins.CRI.Registry.ConfigPath = filepath.Join(constants.CRIConfdPath, "hosts")
	ctrdCfg.Plugins.CRI.Registry.Configs = make(map[string]RegistryConfig)

	for _, registryHost := range slices.Sorted(maps.Keys(r.Config())) {
		hostConfig := r.Config()[registryHost]

		if hostConfig.Auth() != nil {
			cfg := RegistryConfig{}
			cfg.Auth = &AuthConfig{
				Username:      hostConfig.Auth().Username(),
				Password:      hostConfig.Auth().Password(),
				Auth:          hostConfig.Auth().Auth(),
				IdentityToken: hostConfig.Auth().IdentityToken(),
			}

			configHost, _ := docker.DefaultHost(registryHost) //nolint:errcheck // doesn't return an error

			ctrdCfg.Plugins.CRI.Registry.Configs[configHost] = cfg
		}
	}

	var buf bytes.Buffer

	if err := toml.NewEncoder(&buf).SetIndentTables(true).Encode(&ctrdCfg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
