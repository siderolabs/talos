// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

// AuthConfig represents the registry auth options.
type AuthConfig struct {
	Username      string `toml:"username,omitempty"`
	Password      string `toml:"password,omitempty"`
	Auth          string `toml:"auth,omitempty"`
	IdentityToken string `toml:"identitytoken,omitempty"`
}

// RegistryConfig represents a registry.
type RegistryConfig struct {
	Auth *AuthConfig `toml:"auth"`
}

// Registry represents the registry configuration.
type Registry struct {
	ConfigPath string                    `toml:"config_path"`
	Configs    map[string]RegistryConfig `toml:"configs"`
}

// CRIConfig represents the CRI config.
type CRIConfig struct {
	Registry Registry `toml:"registry"`
}

// PluginsConfig represents the CRI plugins config.
type PluginsConfig struct {
	CRI CRIConfig `toml:"io.containerd.grpc.v1.cri"`
}

// Config represnts the containerd config.
type Config struct {
	Plugins PluginsConfig `toml:"plugins"`
}

// RegistryFileConfig represnts the containerd registry config.
type RegistryFileConfig struct {
	Server      string                    `toml:"server"`
	HostConfigs map[string]HostFileConfig `toml:"host"`
}

// HostFileConfig represnts the containerd host config.
type HostFileConfig struct {
	Capabilities []string    `toml:"capabilities,omitempty"`
	CACert       string      `toml:"ca,omitempty"`
	Client       [][2]string `toml:"client,omitempty"`
	SkipVerify   bool        `toml:"skip_verify,omitempty"`
}
