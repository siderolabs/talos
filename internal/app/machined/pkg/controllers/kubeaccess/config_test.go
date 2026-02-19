// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	kubeaccessctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubeaccess"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubeaccess"
)

type ConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ConfigSuite) TestReconcileConfig() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "controlplane",
			MachineFeatures: &v1alpha1.FeaturesConfig{
				KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
					AccessEnabled:                     new(true),
					AccessAllowedRoles:                []string{"os:admin"},
					AccessAllowedKubernetesNamespaces: []string{"kube-system"},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{kubeaccess.ConfigID}, func(r *kubeaccess.Config, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.True(spec.Enabled)
		asrt.Equal([]string{"os:admin"}, spec.AllowedAPIRoles)
		asrt.Equal([]string{"kube-system"}, spec.AllowedKubernetesNamespaces)
	})
}

func (suite *ConfigSuite) TestReconcileDisabled() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "init",
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{kubeaccess.ConfigID}, func(r *kubeaccess.Config, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.False(spec.Enabled)
		asrt.Empty(spec.AllowedAPIRoles)
		asrt.Empty(spec.AllowedKubernetesNamespaces)
	})
}

func (suite *ConfigSuite) TestReconcileWorker() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineFeatures: &v1alpha1.FeaturesConfig{
				KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
					AccessEnabled:                     new(true),
					AccessAllowedRoles:                []string{"os:admin"},
					AccessAllowedKubernetesNamespaces: []string{"kube-system"},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// worker should have feature disabled even if it is enabled in the config
	rtestutils.AssertNoResource[*kubeaccess.Config](suite.Ctx(), suite.T(), suite.State(), kubeaccess.ConfigID)
}

func TestConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(kubeaccessctrl.NewConfigController()))
			},
		},
	})
}
