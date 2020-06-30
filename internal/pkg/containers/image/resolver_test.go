// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
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

type ResolverSuite struct {
	suite.Suite
}

func (suite *ResolverSuite) TestRegistryEndpoints() {
	// defaults
	endpoints, err := image.RegistryEndpoints(&mockConfig{}, "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal([]string{"https://registry-1.docker.io"}, endpoints)

	endpoints, err = image.RegistryEndpoints(&mockConfig{}, "quay.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal([]string{"https://quay.io"}, endpoints)

	// overrides without catch-all
	cfg := &mockConfig{
		mirrors: map[string]runtime.RegistryMirrorConfig{
			"docker.io": {
				Endpoints: []string{"http://127.0.0.1:5000", "https://some.host"},
			},
		},
	}

	endpoints, err = image.RegistryEndpoints(cfg, "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal([]string{"http://127.0.0.1:5000", "https://some.host"}, endpoints)

	endpoints, err = image.RegistryEndpoints(cfg, "quay.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal([]string{"https://quay.io"}, endpoints)

	// overrides with catch-all
	cfg = &mockConfig{
		mirrors: map[string]runtime.RegistryMirrorConfig{
			"docker.io": {
				Endpoints: []string{"http://127.0.0.1:5000", "https://some.host"},
			},
			"*": {
				Endpoints: []string{"http://127.0.0.1:5001"},
			},
		},
	}

	endpoints, err = image.RegistryEndpoints(cfg, "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal([]string{"http://127.0.0.1:5000", "https://some.host"}, endpoints)

	endpoints, err = image.RegistryEndpoints(cfg, "quay.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal([]string{"http://127.0.0.1:5001"}, endpoints)
}

func (suite *ResolverSuite) TestPrepareAuth() {
	user, pass, err := image.PrepareAuth(nil, "docker.io", "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal("", user)
	suite.Assert().Equal("", pass)

	user, pass, err = image.PrepareAuth(&runtime.RegistryAuthConfig{
		Username: "root",
		Password: "secret",
	}, "docker.io", "not.docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal("", user)
	suite.Assert().Equal("", pass)

	user, pass, err = image.PrepareAuth(&runtime.RegistryAuthConfig{
		Username: "root",
		Password: "secret",
	}, "docker.io", "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal("root", user)
	suite.Assert().Equal("secret", pass)

	user, pass, err = image.PrepareAuth(&runtime.RegistryAuthConfig{
		IdentityToken: "xyz",
	}, "docker.io", "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal("", user)
	suite.Assert().Equal("xyz", pass)

	user, pass, err = image.PrepareAuth(&runtime.RegistryAuthConfig{
		Auth: "dXNlcjE6c2VjcmV0MQ==",
	}, "docker.io", "docker.io")
	suite.Assert().NoError(err)
	suite.Assert().Equal("user1", user)
	suite.Assert().Equal("secret1", pass)

	_, _, err = image.PrepareAuth(&runtime.RegistryAuthConfig{}, "docker.io", "docker.io")
	suite.Assert().EqualError(err, "invalid auth config for \"docker.io\"")
}

func (suite *ResolverSuite) TestRegistryHosts() {
	registryHosts, err := image.RegistryHosts(&mockConfig{})("docker.io")
	suite.Require().NoError(err)
	suite.Assert().Len(registryHosts, 1)
	suite.Assert().Equal("https", registryHosts[0].Scheme)
	suite.Assert().Equal("registry-1.docker.io", registryHosts[0].Host)
	suite.Assert().Equal("/v2", registryHosts[0].Path)
	suite.Assert().Nil(registryHosts[0].Client.Transport.(*http.Transport).TLSClientConfig)

	cfg := &mockConfig{
		mirrors: map[string]runtime.RegistryMirrorConfig{
			"docker.io": {
				Endpoints: []string{"http://127.0.0.1:5000/docker.io", "https://some.host"},
			},
		},
	}

	registryHosts, err = image.RegistryHosts(cfg)("docker.io")
	suite.Require().NoError(err)
	suite.Assert().Len(registryHosts, 2)
	suite.Assert().Equal("http", registryHosts[0].Scheme)
	suite.Assert().Equal("127.0.0.1:5000", registryHosts[0].Host)
	suite.Assert().Equal("/docker.io", registryHosts[0].Path)
	suite.Assert().Nil(registryHosts[0].Client.Transport.(*http.Transport).TLSClientConfig)
	suite.Assert().Equal("https", registryHosts[1].Scheme)
	suite.Assert().Equal("some.host", registryHosts[1].Host)
	suite.Assert().Equal("/v2", registryHosts[1].Path)
	suite.Assert().Nil(registryHosts[1].Client.Transport.(*http.Transport).TLSClientConfig)

	cfg = &mockConfig{
		mirrors: map[string]runtime.RegistryMirrorConfig{
			"docker.io": {
				Endpoints: []string{"https://some.host:123"},
			},
		},
		config: map[string]runtime.RegistryConfig{
			"some.host:123": {
				TLS: &runtime.RegistryTLSConfig{
					CA: []byte(caCertMock),
					// ClientIdentity: &x509.PEMEncodedCertificateAndKey{},
				},
				Auth: &runtime.RegistryAuthConfig{
					Username: "root",
					Password: "secret",
				},
			},
		},
	}

	registryHosts, err = image.RegistryHosts(cfg)("docker.io")
	suite.Require().NoError(err)
	suite.Assert().Len(registryHosts, 1)
	suite.Assert().Equal("https", registryHosts[0].Scheme)
	suite.Assert().Equal("some.host:123", registryHosts[0].Host)
	suite.Assert().Equal("/v2", registryHosts[0].Path)

	tlsClientConfig := registryHosts[0].Client.Transport.(*http.Transport).TLSClientConfig
	suite.Require().NotNil(tlsClientConfig)
	suite.Require().NotNil(tlsClientConfig.RootCAs)
	suite.Require().Empty(tlsClientConfig.Certificates)

	suite.Require().NotNil(registryHosts[0].Authorizer)

	req, err := http.NewRequest("GET", "htts://some.host:123/v2", nil)
	suite.Require().NoError(err)

	resp := &http.Response{}
	resp.Request = req
	resp.Header = http.Header{}
	resp.Header.Add("WWW-Authenticate", "Basic realm=\"Access to the staging site\", charset=\"UTF-8\"")

	suite.Require().NoError(registryHosts[0].Authorizer.AddResponses(context.Background(), []*http.Response{resp}))
	suite.Require().NoError(registryHosts[0].Authorizer.Authorize(context.Background(), req))

	suite.Assert().Equal("Basic cm9vdDpzZWNyZXQ=", req.Header.Get("Authorization"))
}

func TestResolverSuite(t *testing.T) {
	suite.Run(t, new(ResolverSuite))
}

const caCertMock = `-----BEGIN CERTIFICATE-----
MIICjTCCAhSgAwIBAgIIdebfy8FoW6gwCgYIKoZIzj0EAwIwfDELMAkGA1UEBhMC
VVMxDjAMBgNVBAgMBVRleGFzMRAwDgYDVQQHDAdIb3VzdG9uMRgwFgYDVQQKDA9T
U0wgQ29ycG9yYXRpb24xMTAvBgNVBAMMKFNTTC5jb20gUm9vdCBDZXJ0aWZpY2F0
aW9uIEF1dGhvcml0eSBFQ0MwHhcNMTYwMjEyMTgxNDAzWhcNNDEwMjEyMTgxNDAz
WjB8MQswCQYDVQQGEwJVUzEOMAwGA1UECAwFVGV4YXMxEDAOBgNVBAcMB0hvdXN0
b24xGDAWBgNVBAoMD1NTTCBDb3Jwb3JhdGlvbjExMC8GA1UEAwwoU1NMLmNvbSBS
b290IENlcnRpZmljYXRpb24gQXV0aG9yaXR5IEVDQzB2MBAGByqGSM49AgEGBSuB
BAAiA2IABEVuqVDEpiM2nl8ojRfLliJkP9x6jh3MCLOicSS6jkm5BBtHllirLZXI
7Z4INcgn64mMU1jrYor+8FsPazFSY0E7ic3s7LaNGdM0B9y7xgZ/wkWV7Mt/qCPg
CemB+vNH06NjMGEwHQYDVR0OBBYEFILRhXMw5zUE044CkvvlpNHEIejNMA8GA1Ud
EwEB/wQFMAMBAf8wHwYDVR0jBBgwFoAUgtGFczDnNQTTjgKS++Wk0cQh6M0wDgYD
VR0PAQH/BAQDAgGGMAoGCCqGSM49BAMCA2cAMGQCMG/n61kRpGDPYbCWe+0F+S8T
kdzt5fxQaxFGRrMcIQBiu77D5+jNB5n5DQtdcj7EqgIwH7y6C+IwJPt8bYBVCpk+
gA0z5Wajs6O7pdWLjwkspl1+4vAHCGht0nxpbl/f5Wpl
-----END CERTIFICATE-----
`
