// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type mockConfig struct {
	mirrors map[string]*v1alpha1.RegistryMirrorConfig
	config  map[string]*v1alpha1.RegistryConfig
}

// Mirrors implements the Registries interface.
func (c *mockConfig) Mirrors() map[string]config.RegistryMirrorConfig {
	mirrors := make(map[string]config.RegistryMirrorConfig, len(c.mirrors))

	for k, v := range c.mirrors {
		mirrors[k] = v
	}

	return mirrors
}

// Config implements the Registries interface.
func (c *mockConfig) Config() map[string]config.RegistryConfig {
	registries := make(map[string]config.RegistryConfig, len(c.config))

	for k, v := range c.config {
		registries[k] = v
	}

	return registries
}

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestGenerateRegistriesConfig() {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []string{"https://registry-1.docker.io", "https://registry-2.docker.io"},
			},
		},
		config: map[string]*v1alpha1.RegistryConfig{
			"some.host:123": {
				RegistryAuth: &v1alpha1.RegistryAuthConfig{
					RegistryUsername:      "root",
					RegistryPassword:      "secret",
					RegistryAuth:          "auth",
					RegistryIdentityToken: "token",
				},
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSInsecureSkipVerify: true,
					TLSCA:                 []byte("cacert"),
					TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("clientcert"),
						Key: []byte("clientkey"),
					},
				},
			},
		},
	}

	files, err := containerd.GenerateRegistriesConfig(cfg)
	suite.Require().NoError(err)
	suite.Assert().Equal([]config.File{
		&v1alpha1.MachineFile{
			FileContent: `server = "https://docker.io"

[host]
  [host."https://registry-1.docker.io"]
    capabilities = ["pull", "resolve"]
  [host."https://registry-2.docker.io"]
    capabilities = ["pull", "resolve"]
`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/docker.io/hosts.toml",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent:     `cacert`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/some.host:123/some.host:123-ca.crt",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent:     `clientcert`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/some.host:123/some.host:123.crt",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent:     `clientkey`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/some.host:123/some.host:123.key",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent: `server = "https://some.host:123"

[host]
  [host."https://some.host:123"]
    ca = "/var/etc/cri/conf.d/some.host:123/some.host:123-ca.crt"
    client = [["/var/etc/cri/conf.d/some.host:123/some.host:123.crt", "/var/etc/cri/conf.d/some.host:123/some.host:123.key"]]
    skip_verify = true
`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/some.host:123/hosts.toml",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent: `[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/var/etc/cri/conf.d"
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."some.host:123"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs."some.host:123".auth]
            username = "root"
            password = "secret"
            auth = "auth"
            identitytoken = "token"
`,
			FilePermissions: 0o644,
			FilePath:        constants.CRIContainerdConfigDir + "/registry.toml",
			FileOp:          "create",
		},
	}, files)
}

func (suite *ConfigSuite) TestGenerateRegistriesInsecureConfig() {
	cfg := &mockConfig{
		mirrors: map[string]*v1alpha1.RegistryMirrorConfig{
			"myregistrydomain.com": {
				MirrorEndpoints: []string{"http://myregistrydomain.local", "https://myregistrydomain.com"},
			},
		},
		config: map[string]*v1alpha1.RegistryConfig{
			"http://myregistrydomain.local": {
				RegistryAuth: &v1alpha1.RegistryAuthConfig{
					RegistryUsername: "root",
					RegistryPassword: "secret",
				},
			},
			"myregistrydomain.com": {
				RegistryTLS: &v1alpha1.RegistryTLSConfig{
					TLSCA: []byte("cacert"),
				},
			},
		},
	}

	files, err := containerd.GenerateRegistriesConfig(cfg)
	suite.Require().NoError(err)
	suite.Assert().Equal([]config.File{
		&v1alpha1.MachineFile{
			FileContent:     `cacert`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/myregistrydomain.com/myregistrydomain.com-ca.crt",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent: `server = "https://myregistrydomain.com"

[host]
  [host."http://myregistrydomain.local"]
    capabilities = ["pull", "resolve"]
  [host."https://myregistrydomain.com"]
    capabilities = ["pull", "resolve"]
    ca = "/var/etc/cri/conf.d/myregistrydomain.com/myregistrydomain.com-ca.crt"
`,
			FilePermissions: 0o600,
			FilePath:        constants.CRIContainerdConfigDir + "/myregistrydomain.com/hosts.toml",
			FileOp:          "create",
		},
		&v1alpha1.MachineFile{
			FileContent: `[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/var/etc/cri/conf.d"
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."myregistrydomain.local"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs."myregistrydomain.local".auth]
            username = "root"
            password = "secret"
`,
			FilePermissions: 0o644,
			FilePath:        constants.CRIContainerdConfigDir + "/registry.toml",
			FileOp:          "create",
		},
	}, files)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
