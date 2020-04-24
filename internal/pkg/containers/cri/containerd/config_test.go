// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/universe"
)

type mockConfig struct {
	mirrors map[string]runtime.RegistryMirrorConfig
	config  map[string]runtime.RegistryConfig
}

func (c *mockConfig) Mirrors() map[string]runtime.RegistryMirrorConfig {
	return c.mirrors
}

func (c *mockConfig) Config() map[string]runtime.RegistryConfig {
	return c.config
}

func (c *mockConfig) ExtraFiles() ([]runtime.File, error) {
	return nil, fmt.Errorf("not implemented")
}

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestGenerateRegistriesConfig() {
	cfg := &mockConfig{
		mirrors: map[string]runtime.RegistryMirrorConfig{
			"docker.io": {
				Endpoints: []string{"https://registry-1.docker.io", "https://registry-2.docker.io"},
			},
		},
		config: map[string]runtime.RegistryConfig{
			"some.host:123": {
				Auth: &runtime.RegistryAuthConfig{
					Username:      "root",
					Password:      "secret",
					Auth:          "auth",
					IdentityToken: "token",
				},
				TLS: &runtime.RegistryTLSConfig{
					InsecureSkipVerify: true,
					CA:                 []byte("cacert"),
					ClientIdentity: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("clientcert"),
						Key: []byte("clientkey"),
					},
				},
			},
		},
	}

	files, err := containerd.GenerateRegistriesConfig(cfg)
	suite.Require().NoError(err)
	suite.Assert().Equal([]runtime.File{
		{
			Content:     `cacert`,
			Permissions: 0600,
			Path:        "/etc/cri/ca/some.host:123.crt",
			Op:          "create",
		},
		{
			Content:     `clientcert`,
			Permissions: 0600,
			Path:        "/etc/cri/client/some.host:123.crt",
			Op:          "create",
		},
		{
			Content:     `clientkey`,
			Permissions: 0600,
			Path:        "/etc/cri/client/some.host:123.key",
			Op:          "create",
		},
		{
			Content: `[plugins]
  [plugins.cri]
    [plugins.cri.registry]
      [plugins.cri.registry.mirrors]
        [plugins.cri.registry.mirrors."docker.io"]
          endpoint = ["https://registry-1.docker.io", "https://registry-2.docker.io"]
      [plugins.cri.registry.configs]
        [plugins.cri.registry.configs."some.host:123"]
          [plugins.cri.registry.configs."some.host:123".auth]
            username = "root"
            password = "secret"
            auth = "auth"
            identitytoken = "token"
          [plugins.cri.registry.configs."some.host:123".tls]
            insecure_skip_verify = true
            ca_file = "/etc/cri/ca/some.host:123.crt"
            cert_file = "/etc/cri/client/some.host:123.crt"
            key_file = "/etc/cri/client/some.host:123.key"
`,
			Permissions: 0644,
			Path:        universe.CRIContainerdConfig,
			Op:          "append",
		},
	}, files)
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
