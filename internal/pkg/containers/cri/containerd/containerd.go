// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerd provides support for containerd CRI plugin
package containerd

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// GenerateHostsTLSConfig returns a HostFileConfig.
//
func GenerateHostsTLSConfig(h HostFileConfig, r config.RegistryTLSConfig, registryHost string) ([]config.File, HostFileConfig) {
	hostFile := h

	extraFiles := []config.File{}
	hostFile.SkipVerify = r.InsecureSkipVerify()

	if r.CA() != nil {
		path := filepath.Join(constants.CRIContainerdConfigDir, registryHost, fmt.Sprintf("%s-ca.crt", registryHost))
		hostFile.CACert = path

		extraFiles = append(extraFiles, &v1alpha1.MachineFile{
			FileContent:     string(r.CA()),
			FilePermissions: 0o600,
			FilePath:        path,
			FileOp:          "create",
		})
	}

	if r.ClientIdentity() != nil {
		clientPair := [2]string{}

		if r.ClientIdentity().Crt != nil {
			path := filepath.Join(constants.CRIContainerdConfigDir, registryHost, fmt.Sprintf("%s.crt", registryHost))

			extraFiles = append(extraFiles, &v1alpha1.MachineFile{
				FileContent:     string(r.ClientIdentity().Crt),
				FilePermissions: 0o600,
				FilePath:        path,
				FileOp:          "create",
			})

			clientPair[0] = path
		}

		if r.ClientIdentity().Key != nil {
			path := filepath.Join(constants.CRIContainerdConfigDir, registryHost, fmt.Sprintf("%s.key", registryHost))

			extraFiles = append(extraFiles, &v1alpha1.MachineFile{
				FileContent:     string(r.ClientIdentity().Key),
				FilePermissions: 0o600,
				FilePath:        path,
				FileOp:          "create",
			})

			clientPair[1] = path
		}

		if r.ClientIdentity().Crt != nil || r.ClientIdentity().Key != nil {
			hostFile.Client = [][2]string{}
			hostFile.Client = append(hostFile.Client, clientPair)
		}
	}

	return extraFiles, hostFile
}

// GenerateRegistriesConfig returns a list of extra files.
//
//nolint:gocyclo
func GenerateRegistriesConfig(r config.Registries) ([]config.File, error) {
	ctrdCfg := Config{}
	ctrdCfg.Plugins.CRI.Registry.ConfigPath = constants.CRIContainerdConfigDir
	ctrdCfg.Plugins.CRI.Registry.Configs = make(map[string]RegistryConfig)

	extraFiles := []config.File{}
	registryConfig := r.Config()

	for mirrorName, mirrorConfig := range r.Mirrors() {
		mirrorNametURL := mirrorName
		if !strings.HasPrefix(mirrorNametURL, "http") {
			mirrorNametURL = "https://" + mirrorName
		}

		mirrorCfg := RegistryFileConfig{}
		mirrorCfg.Server = mirrorNametURL
		mirrorCfg.HostConfigs = make(map[string]HostFileConfig)

		for _, mirror := range mirrorConfig.Endpoints() {
			u, err := url.Parse(mirror)
			if err != nil {
				return nil, err
			}

			mirrorKey := mirror
			if strings.HasPrefix(mirror, "https") {
				mirrorKey = u.Host
			}

			hostCfg := HostFileConfig{
				Capabilities: []string{"pull", "resolve"},
			}

			if registryConfig[mirrorKey] != nil {
				if registryConfig[mirrorKey].TLS() != nil {
					var files []config.File

					files, hostCfg = GenerateHostsTLSConfig(hostCfg, registryConfig[mirrorKey].TLS(), u.Host)
					extraFiles = append(extraFiles, files...)
				}

				if registryConfig[mirrorKey].Auth() != nil {
					cfg := RegistryConfig{}
					auth := registryConfig[mirrorKey].Auth()
					cfg.Auth = &AuthConfig{
						Username:      auth.Username(),
						Password:      auth.Password(),
						Auth:          auth.Auth(),
						IdentityToken: auth.IdentityToken(),
					}
					ctrdCfg.Plugins.CRI.Registry.Configs[u.Host] = cfg
				}

				delete(registryConfig, mirrorKey)
			}

			mirrorCfg.HostConfigs[mirror] = hostCfg
		}

		var buf bytes.Buffer

		if err := toml.NewEncoder(&buf).Encode(&mirrorCfg); err != nil {
			return nil, err
		}

		extraFiles = append(extraFiles, &v1alpha1.MachineFile{
			FileContent:     buf.String(),
			FilePermissions: 0o600,
			FilePath:        filepath.Join(constants.CRIContainerdConfigDir, mirrorName, "hosts.toml"),
			FileOp:          "create",
		})
	}

	for registryHost, hostConfig := range registryConfig {
		cfg := RegistryConfig{}

		registryHostURL := registryHost
		if !strings.HasPrefix(registryHostURL, "http") {
			registryHostURL = "https://" + registryHost
		}

		u, err := url.Parse(registryHostURL)
		if err != nil {
			return nil, err
		}

		if hostConfig.Auth() != nil {
			cfg.Auth = &AuthConfig{
				Username:      hostConfig.Auth().Username(),
				Password:      hostConfig.Auth().Password(),
				Auth:          hostConfig.Auth().Auth(),
				IdentityToken: hostConfig.Auth().IdentityToken(),
			}
		}

		registryCfg := RegistryFileConfig{}
		registryCfg.Server = registryHostURL
		registryCfg.HostConfigs = make(map[string]HostFileConfig)

		if hostConfig.TLS() != nil {
			files, hostCfg := GenerateHostsTLSConfig(HostFileConfig{}, hostConfig.TLS(), u.Host)
			extraFiles = append(extraFiles, files...)

			registryCfg.HostConfigs[registryHostURL] = hostCfg
		}

		var buf bytes.Buffer

		if err := toml.NewEncoder(&buf).Encode(&registryCfg); err != nil {
			return nil, err
		}

		extraFiles = append(extraFiles, &v1alpha1.MachineFile{
			FileContent:     buf.String(),
			FilePermissions: 0o600,
			FilePath:        filepath.Join(constants.CRIContainerdConfigDir, u.Host, "hosts.toml"),
			FileOp:          "create",
		})

		if cfg.Auth != nil {
			ctrdCfg.Plugins.CRI.Registry.Configs[u.Host] = cfg
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
		FilePath:        constants.CRIContainerdConfigDir + "/registry.toml",
		FileOp:          "create",
	}), nil
}
