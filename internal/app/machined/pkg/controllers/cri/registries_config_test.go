// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"testing"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
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
					"docker.io": {MirrorEndpoints: []string{"https://mirror.io"}},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, crires.RegistriesConfigID, func(r *crires.RegistriesConfig, a *assert.Assertions) {
		spec := r.TypedSpec()

		a.Equal(
			map[string]*crires.RegistryMirrorConfig{
				"docker.io": {MirrorEndpoints: []string{"https://mirror.io"}},
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
				"*": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
				}},
				"docker.io": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
					"https://mirror.io",
				}},
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
				"docker.io": {MirrorEndpoints: []string{"https://mirror.io"}},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryConfig{
				"docker.io": {
					RegistryAuth: &crires.RegistryAuthConfig{
						RegistryUsername:      "example",
						RegistryPassword:      "pass",
						RegistryAuth:          "someauth",
						RegistryIdentityToken: "token",
					},
				},
			},
			spec.RegistryConfig,
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
				"*": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
				}},
				"docker.io": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
					"https://mirror.io",
				}},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryConfig{
				"docker.io": {
					RegistryAuth: &crires.RegistryAuthConfig{
						RegistryUsername:      "example",
						RegistryPassword:      "pass",
						RegistryAuth:          "someauth",
						RegistryIdentityToken: "token",
					},
				},
			},
			spec.RegistryConfig,
		)
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
							TLSInsecureSkipVerify: pointer.To(true),
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
				"docker.io": {MirrorEndpoints: []string{"https://mirror.io"}},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryConfig{
				"docker.io": {
					RegistryTLS: &crires.RegistryTLSConfig{
						TLSInsecureSkipVerify: pointer.To(true),
					},
				},
			},
			spec.RegistryConfig,
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
				"*": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
				}},
				"docker.io": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
					"https://mirror.io",
				}},
			},
			spec.RegistryMirrors,
		)

		a.Equal(
			map[string]*crires.RegistryConfig{
				"docker.io": {
					RegistryTLS: &crires.RegistryTLSConfig{
						TLSInsecureSkipVerify: pointer.To(true),
					},
				},
			},
			spec.RegistryConfig,
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
				"*": {MirrorEndpoints: []string{
					"http://" + constants.RegistrydListenAddress,
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
