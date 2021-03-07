// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

// Mirror represents a registry mirror.
type Mirror struct {
	Endpoints []string `toml:"endpoint"`
}

// AuthConfig represents the registry auth options.
type AuthConfig struct {
	Username      string `toml:"username"`
	Password      string `toml:"password"`
	Auth          string `toml:"auth"`
	IdentityToken string `toml:"identitytoken"`
}

// TLSConfig represents the registry TLS options.
type TLSConfig struct {
	InsecureSkipVerify bool   `toml:"insecure_skip_verify"`
	CAFile             string `toml:"ca_file"`
	CertFile           string `toml:"cert_file"`
	KeyFile            string `toml:"key_file"`
}

// RegistryConfig represents a registry.
type RegistryConfig struct {
	Auth *AuthConfig `toml:"auth"`
	TLS  *TLSConfig  `toml:"tls"`
}

// Registry represents the registry configuration.
type Registry struct {
	Mirrors map[string]Mirror         `toml:"mirrors"`
	Configs map[string]RegistryConfig `toml:"configs"`
}

// CRIConfig represents the CRI config.
type CRIConfig struct {
	Registry Registry `toml:"registry"`
}

// PluginsConfig represents the CRI plugins config.
type PluginsConfig struct {
	CRI CRIConfig `toml:"cri"`
}

// Config represnts the containerd config.
type Config struct {
	Plugins PluginsConfig `toml:"plugins"`
}
