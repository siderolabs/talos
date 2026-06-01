// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	criconfig "github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

type ConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ConfigSuite) TestRegistry() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "controlplane",
			MachineRegistries: v1alpha1.RegistriesConfig{
				RegistryMirrors: map[string]*v1alpha1.RegistryMirrorConfig{
					"docker.io": {
						MirrorEndpoints:    []string{"https://mirror.io"},
						MirrorOverridePath: new(true),
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"docker.io": {
					MirrorEndpoints: []crires.RegistryEndpointConfig{
						{
							EndpointEndpoint:     "https://mirror.io",
							EndpointOverridePath: true,
						},
					},
				},
			},
			spec.RegistryMirrors,
		)
	})

	ic := crires.NewImageCacheConfig()
	ic.TypedSpec().Roots = []string{"/imagecache"}
	ic.TypedSpec().Status = crires.ImageCacheStatusReady

	suite.Require().NoError(suite.State().Create(suite.Ctx(), ic))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"*": {
					MirrorEndpoints: []crires.RegistryEndpointConfig{
						{
							EndpointEndpoint: "http://" + constants.RegistrydListenAddress,
						},
					},
				},
				"docker.io": {
					MirrorEndpoints: []crires.RegistryEndpointConfig{
						{
							EndpointEndpoint: "http://" + constants.RegistrydListenAddress,
						},
						{
							EndpointEndpoint:     "https://mirror.io",
							EndpointOverridePath: true,
						},
					},
				},
			},
			spec.RegistryMirrors,
		)
	})
}

func (suite *ConfigSuite) TestRegistryAuth() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "controlplane",
			MachineRegistries: v1alpha1.RegistriesConfig{
				RegistryMirrors: map[string]*v1alpha1.RegistryMirrorConfig{
					"docker.io": {MirrorEndpoints: []string{"https://mirror.io"}},
				},
				RegistryConfig: map[string]*v1alpha1.RegistryConfig{
					"docker.io": {
						RegistryAuth: &v1alpha1.RegistryAuthConfig{
							RegistryUsername:      "example",
							RegistryPassword:      "pass",
							RegistryAuth:          "someauth",
							RegistryIdentityToken: "token",
						},
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"docker.io": {MirrorEndpoints: []crires.RegistryEndpointConfig{{EndpointEndpoint: "https://mirror.io"}}},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryAuthConfig{
				"docker.io": {
					RegistryUsername:      "example",
					RegistryPassword:      "pass",
					RegistryAuth:          "someauth",
					RegistryIdentityToken: "token",
				},
			},
			spec.RegistryAuths,
		)

		a.Empty(spec.RegistryTLSs)
	})

	ic := crires.NewImageCacheConfig()
	ic.TypedSpec().Roots = []string{"/imagecache"}
	ic.TypedSpec().Status = crires.ImageCacheStatusReady

	suite.Require().NoError(suite.State().Create(suite.Ctx(), ic))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"*": {MirrorEndpoints: []crires.RegistryEndpointConfig{
					{EndpointEndpoint: "http://" + constants.RegistrydListenAddress},
				}},
				"docker.io": {MirrorEndpoints: []crires.RegistryEndpointConfig{
					{EndpointEndpoint: "http://" + constants.RegistrydListenAddress},
					{EndpointEndpoint: "https://mirror.io"},
				}},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryAuthConfig{
				"docker.io": {
					RegistryUsername:      "example",
					RegistryPassword:      "pass",
					RegistryAuth:          "someauth",
					RegistryIdentityToken: "token",
				},
			},
			spec.RegistryAuths,
		)

		a.Empty(spec.RegistryTLSs)
	})
}

func (suite *ConfigSuite) TestRegistryTLS() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "controlplane",
			MachineRegistries: v1alpha1.RegistriesConfig{
				RegistryMirrors: map[string]*v1alpha1.RegistryMirrorConfig{
					"docker.io": {MirrorEndpoints: []string{"https://mirror.io"}},
				},
				RegistryConfig: map[string]*v1alpha1.RegistryConfig{
					"docker.io": {
						RegistryTLS: &v1alpha1.RegistryTLSConfig{
							TLSInsecureSkipVerify: new(true),
						},
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"docker.io": {MirrorEndpoints: []crires.RegistryEndpointConfig{{EndpointEndpoint: "https://mirror.io"}}},
			},
			spec.RegistryMirrors,
		)

		a.Empty(spec.RegistryAuths)

		a.Equal(
			map[string]*crires.RegistryTLSConfig{
				"docker.io": {
					TLSInsecureSkipVerify: true,
				},
			},
			spec.RegistryTLSs,
		)
	})

	ic := crires.NewImageCacheConfig()
	ic.TypedSpec().Roots = []string{"/imagecache"}
	ic.TypedSpec().Status = crires.ImageCacheStatusReady

	suite.Require().NoError(suite.State().Create(suite.Ctx(), ic))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"*": {MirrorEndpoints: []crires.RegistryEndpointConfig{
					{EndpointEndpoint: "http://" + constants.RegistrydListenAddress},
				}},
				"docker.io": {MirrorEndpoints: []crires.RegistryEndpointConfig{
					{EndpointEndpoint: "http://" + constants.RegistrydListenAddress},
					{EndpointEndpoint: "https://mirror.io"},
				}},
			},
			spec.RegistryMirrors,
		)

		a.Empty(spec.RegistryAuths)

		a.Equal(
			map[string]*crires.RegistryTLSConfig{
				"docker.io": {
					TLSInsecureSkipVerify: true,
				},
			},
			spec.RegistryTLSs,
		)
	})
}

func (suite *ConfigSuite) TestRegistryImageCacheNoConfig() {
	ic := crires.NewImageCacheConfig()
	ic.TypedSpec().Roots = []string{"/imagecache"}
	ic.TypedSpec().Status = crires.ImageCacheStatusReady

	suite.Require().NoError(suite.State().Create(suite.Ctx(), ic))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"*": {MirrorEndpoints: []crires.RegistryEndpointConfig{
					{EndpointEndpoint: "http://" + constants.RegistrydListenAddress},
				}},
			},
			spec.RegistryMirrors,
		)
	})
}

func (suite *ConfigSuite) TestRegistryNoConfig() {
	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Empty(
			spec.RegistryMirrors,
		)
	})
}

func (suite *ConfigSuite) TestRegistryNewStyle() {
	mr1 := criconfig.NewRegistryMirrorConfigV1Alpha1("docker.io")
	mr1.RegistryEndpoints = []criconfig.RegistryEndpoint{
		{
			EndpointURL:          meta.URL{URL: ensure.Value(url.Parse("https://mirror1.io"))},
			EndpointOverridePath: new(true),
		},
		{
			EndpointURL: meta.URL{URL: ensure.Value(url.Parse("https://mirror2.io"))},
		},
	}

	ar1 := criconfig.NewRegistryAuthConfigV1Alpha1("registry-1.docker.io")
	ar1.RegistryUsername = "docker-example"
	ar1.RegistryPassword = "docker-pass"

	tr1 := criconfig.NewRegistryTLSConfigV1Alpha1("private-registry:3000")
	tr1.TLSInsecureSkipVerify = new(true)
	tr1.TLSClientIdentity = &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN PRIVATE KEY-----\nMIIE...AB\n-----END PRIVATE KEY-----",
	}
	tr1.TLSCA = "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----"

	tr2 := criconfig.NewRegistryTLSConfigV1Alpha1("another-registry")
	tr2.TLSInsecureSkipVerify = new(true)

	ctr, err := container.New(mr1, ar1, tr1, tr2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"docker.io": {
					MirrorEndpoints: []crires.RegistryEndpointConfig{
						{
							EndpointEndpoint:     "https://mirror1.io",
							EndpointOverridePath: true,
						},
						{
							EndpointEndpoint: "https://mirror2.io",
						},
					},
				},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryAuthConfig{
				"registry-1.docker.io": {
					RegistryUsername: "docker-example",
					RegistryPassword: "docker-pass",
				},
			},
			spec.RegistryAuths,
		)

		a.Equal(
			map[string]*crires.RegistryTLSConfig{
				"private-registry:3000": {
					TLSInsecureSkipVerify: true,
					TLSClientIdentity: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----"),
						Key: []byte("-----BEGIN PRIVATE KEY-----\nMIIE...AB\n-----END PRIVATE KEY-----"),
					},
					TLSCA: []byte("-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----"),
				},
				"another-registry": {
					TLSInsecureSkipVerify: true,
				},
			},
			spec.RegistryTLSs,
		)
	})
}

func TestConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&cri.RegistriesConfigController{}))
			},
		},
	})
}
