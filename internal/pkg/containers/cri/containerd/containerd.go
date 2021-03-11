// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerd provides support for containerd CRI plugin
package containerd

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// GenerateRegistriesConfig returns a list of extra files.
//
//nolint:gocyclo
func GenerateRegistriesConfig(r config.Registries) ([]config.File, error) {
	caPath := filepath.Join("/var", filepath.Dir(constants.CRIContainerdConfig), "ca")
	clientPath := filepath.Join("/var", filepath.Dir(constants.CRIContainerdConfig), "client")

	var ctrdCfg Config
	ctrdCfg.Plugins.CRI.Registry.Mirrors = make(map[string]Mirror)
	ctrdCfg.Plugins.CRI.Registry.Configs = make(map[string]RegistryConfig)

	for mirrorName, mirrorConfig := range r.Mirrors() {
		ctrdCfg.Plugins.CRI.Registry.Mirrors[mirrorName] = Mirror{Endpoints: mirrorConfig.Endpoints()}
	}

	var extraFiles []config.File

	for registryHost, hostConfig := range r.Config() {
		cfg := RegistryConfig{}

		if hostConfig.Auth() != nil {
			cfg.Auth = &AuthConfig{
				Username:      hostConfig.Auth().Username(),
				Password:      hostConfig.Auth().Password(),
				Auth:          hostConfig.Auth().Auth(),
				IdentityToken: hostConfig.Auth().IdentityToken(),
			}
		}

		if hostConfig.TLS() != nil {
			cfg.TLS = &TLSConfig{
				InsecureSkipVerify: hostConfig.TLS().InsecureSkipVerify(),
			}

			if hostConfig.TLS().CA() != nil {
				path := filepath.Join(caPath, fmt.Sprintf("%s.crt", registryHost))

				extraFiles = append(extraFiles, &v1alpha1.MachineFile{
					FileContent:     string(hostConfig.TLS().CA()),
					FilePermissions: 0o600,
					FilePath:        path,
					FileOp:          "create",
				})

				cfg.TLS.CAFile = path
			}

			if hostConfig.TLS().ClientIdentity() != nil && hostConfig.TLS().ClientIdentity().Crt != nil {
				path := filepath.Join(clientPath, fmt.Sprintf("%s.crt", registryHost))

				extraFiles = append(extraFiles, &v1alpha1.MachineFile{
					FileContent:     string(hostConfig.TLS().ClientIdentity().Crt),
					FilePermissions: 0o600,
					FilePath:        path,
					FileOp:          "create",
				})

				cfg.TLS.CertFile = path
			}

			if hostConfig.TLS().ClientIdentity() != nil && hostConfig.TLS().ClientIdentity().Key != nil {
				path := filepath.Join(clientPath, fmt.Sprintf("%s.key", registryHost))

				extraFiles = append(extraFiles, &v1alpha1.MachineFile{
					FileContent:     string(hostConfig.TLS().ClientIdentity().Key),
					FilePermissions: 0o600,
					FilePath:        path,
					FileOp:          "create",
				})

				cfg.TLS.KeyFile = path
			}
		}

		if cfg.Auth != nil || cfg.TLS != nil {
			ctrdCfg.Plugins.CRI.Registry.Configs[registryHost] = cfg
		}
	}

	var buf bytes.Buffer

	if err := toml.NewEncoder(&buf).Encode(&ctrdCfg); err != nil {
		return nil, err
	}

	// CRI plugin doesn't support merging configs for plugins across files,
	// so we have to append CRI plugin to the main config, as it already contains
	// configuration pieces for CRI plugin
	return append(extraFiles, &v1alpha1.MachineFile{
		FileContent:     buf.String(),
		FilePermissions: 0o644,
		FilePath:        constants.CRIContainerdConfig,
		FileOp:          "append",
	}), nil
}
