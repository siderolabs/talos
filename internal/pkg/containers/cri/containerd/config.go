// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// config structures to generate TOML containerd CRI plugin config
type mirror struct {
	Endpoints []string `toml:"endpoint"`
}

type authConfig struct {
	Username      string `toml:"username"`
	Password      string `toml:"password"`
	Auth          string `toml:"auth"`
	IdentityToken string `toml:"identitytoken"`
}

type tlsConfig struct {
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
	CAFile             string `toml:"ca_file"`
	CertFile           string `toml:"cert_file"`
	KeyFile            string `toml:"key_file"`
}

type registryConfig struct {
	Auth *authConfig `toml:"auth"`
	TLS  *tlsConfig  `toml:"tls"`
}

type registry struct {
	Mirrors map[string]mirror         `toml:"mirrors"`
	Configs map[string]registryConfig `toml:"configs"`
}

type criConfig struct {
	Registry registry `toml:"registry"`
}

type pluginsConfig struct {
	CRI criConfig `toml:"cri"`
}

type containerdConfig struct {
	Plugins pluginsConfig `toml:"plugins"`
}

// GenerateRegistriesConfig for containerd CRI plugin (TOML format).
//
//nolint: gocyclo
func GenerateRegistriesConfig(input runtime.Registries) ([]runtime.File, error) {
	caPath := filepath.Join(filepath.Dir(constants.CRIContainerdConfig), "ca")
	clientPath := filepath.Join(filepath.Dir(constants.CRIContainerdConfig), "client")

	var config containerdConfig
	config.Plugins.CRI.Registry.Mirrors = make(map[string]mirror)
	config.Plugins.CRI.Registry.Configs = make(map[string]registryConfig)

	for mirrorName, mirrorConfig := range input.Mirrors() {
		config.Plugins.CRI.Registry.Mirrors[mirrorName] = mirror{Endpoints: mirrorConfig.Endpoints}
	}

	var extraFiles []runtime.File

	for registryHost, hostConfig := range input.Config() {
		cfg := registryConfig{}

		if hostConfig.Auth != nil {
			cfg.Auth = &authConfig{
				Username:      hostConfig.Auth.Username,
				Password:      hostConfig.Auth.Password,
				Auth:          hostConfig.Auth.Auth,
				IdentityToken: hostConfig.Auth.IdentityToken,
			}
		}

		if hostConfig.TLS != nil {
			cfg.TLS = &tlsConfig{
				InsecureSkipVerify: hostConfig.TLS.InsecureSkipVerify,
			}

			if hostConfig.TLS.CA != nil {
				path := filepath.Join(caPath, fmt.Sprintf("%s.crt", registryHost))

				extraFiles = append(extraFiles, runtime.File{
					Content:     string(hostConfig.TLS.CA),
					Permissions: 0600,
					Path:        path,
					Op:          "create",
				})

				cfg.TLS.CAFile = path
			}

			if hostConfig.TLS.ClientIdentity.Crt != nil {
				path := filepath.Join(clientPath, fmt.Sprintf("%s.crt", registryHost))

				extraFiles = append(extraFiles, runtime.File{
					Content:     string(hostConfig.TLS.ClientIdentity.Crt),
					Permissions: 0600,
					Path:        path,
					Op:          "create",
				})

				cfg.TLS.CertFile = path
			}

			if hostConfig.TLS.ClientIdentity.Key != nil {
				path := filepath.Join(clientPath, fmt.Sprintf("%s.key", registryHost))

				extraFiles = append(extraFiles, runtime.File{
					Content:     string(hostConfig.TLS.ClientIdentity.Key),
					Permissions: 0600,
					Path:        path,
					Op:          "create",
				})

				cfg.TLS.KeyFile = path
			}
		}

		if cfg.Auth != nil || cfg.TLS != nil {
			config.Plugins.CRI.Registry.Configs[registryHost] = cfg
		}
	}

	var buf bytes.Buffer

	if err := toml.NewEncoder(&buf).Encode(&config); err != nil {
		return nil, err
	}

	// CRI plugin doesn't support merging configs for plugins across files,
	// so we have to append CRI plugin to the main config, as it already contains
	// configuration pieces for CRI plugin
	return append(extraFiles, runtime.File{
		Content:     buf.String(),
		Permissions: 0644,
		Path:        constants.CRIContainerdConfig,
		Op:          "append",
	}), nil
}
