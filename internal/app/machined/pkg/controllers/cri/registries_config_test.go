// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"testing"
	"time"

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

func (suite *ConfigSuite) TestRegistryNoMachineConfig() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(nil))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

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

func TestConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(cri.NewRegistriesConfigController()))
			},
		},
	})
}
