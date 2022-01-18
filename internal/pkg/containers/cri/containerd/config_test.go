// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/cri.toml
var expectedCRIConfig string

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

	criConfig, err := containerd.GenerateCRIConfig(cfg)
	suite.Require().NoError(err)

	suite.Assert().Equal(expectedCRIConfig, string(criConfig))
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
