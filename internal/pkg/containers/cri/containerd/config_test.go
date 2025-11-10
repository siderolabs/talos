// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/pkg/containers/cri/containerd"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

//go:embed testdata/cri.toml
var expectedCRIConfig string

type mockConfig struct {
	mirrors map[string]*cri.RegistryMirrorConfig
	auths   map[string]*cri.RegistryAuthConfig
	tlses   map[string]*cri.RegistryTLSConfig
}

// Mirrors implements the Registries interface.
func (c *mockConfig) Mirrors() map[string]config.RegistryMirrorConfig {
	mirrors := make(map[string]config.RegistryMirrorConfig, len(c.mirrors))

	for k, v := range c.mirrors {
		mirrors[k] = v
	}

	return mirrors
}

// Auths implements the Registries interface.
func (c *mockConfig) Auths() map[string]config.RegistryAuthConfig {
	auths := make(map[string]config.RegistryAuthConfig, len(c.auths))

	for k, v := range c.auths {
		auths[k] = v
	}

	return auths
}

// TLSs implements the Registries interface.
func (c *mockConfig) TLSs() map[string]cri.RegistryTLSConfigExtended {
	tlses := make(map[string]cri.RegistryTLSConfigExtended, len(c.tlses))

	for k, v := range c.tlses {
		tlses[k] = v
	}

	return tlses
}

type ConfigSuite struct {
	suite.Suite
}

func (suite *ConfigSuite) TestGenerateRegistriesConfig() {
	cfg := &mockConfig{
		mirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{
					{EndpointEndpoint: "https://registry-1.docker.io"},
					{EndpointEndpoint: "https://registry-2.docker.io"},
				},
			},
		},
		auths: map[string]*cri.RegistryAuthConfig{
			"some.host:123": {
				RegistryUsername:      "root",
				RegistryPassword:      "secret",
				RegistryAuth:          "auth",
				RegistryIdentityToken: "token",
			},
			"docker.io": {
				RegistryUsername: "root",
				RegistryPassword: "topsecret",
			},
		},
		tlses: map[string]*cri.RegistryTLSConfig{
			"some.host:123": {
				TLSInsecureSkipVerify: true,
				TLSCA:                 []byte("cacert"),
				TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
					Crt: []byte("clientcert"),
					Key: []byte("clientkey"),
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
