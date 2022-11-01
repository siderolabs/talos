// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	kubeaccessctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/kubeaccess"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubeaccess"
)

type ConfigSuite struct {
	KubeaccessSuite
}

func (suite *ConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&kubeaccessctrl.ConfigController{}))

	suite.startRuntime()

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)

	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
					AccessEnabled:                     pointer.To(true),
					AccessAllowedRoles:                []string{"os:admin"},
					AccessAllowedKubernetesNamespaces: []string{"kube-system"},
				},
			},
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(config.NamespaceName, kubeaccess.ConfigType, kubeaccess.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*kubeaccess.Config).TypedSpec()

				suite.Assert().True(spec.Enabled)
				suite.Assert().Equal([]string{"os:admin"}, spec.AllowedAPIRoles)
				suite.Assert().Equal([]string{"kube-system"}, spec.AllowedKubernetesNamespaces)

				return nil
			},
		),
	))
}

func (suite *ConfigSuite) TestReconcileDisabled() {
	suite.Require().NoError(suite.runtime.RegisterController(&kubeaccessctrl.ConfigController{}))

	suite.startRuntime()

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeInit)

	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(config.NamespaceName, kubeaccess.ConfigType, kubeaccess.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*kubeaccess.Config).TypedSpec()

				suite.Assert().False(spec.Enabled)
				suite.Assert().Empty(spec.AllowedAPIRoles)
				suite.Assert().Empty(spec.AllowedKubernetesNamespaces)

				return nil
			},
		),
	))
}

func (suite *ConfigSuite) TestReconcileWorker() {
	suite.Require().NoError(suite.runtime.RegisterController(&kubeaccessctrl.ConfigController{}))

	suite.startRuntime()

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)

	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{
					AccessEnabled:                     pointer.To(true),
					AccessAllowedRoles:                []string{"os:admin"},
					AccessAllowedKubernetesNamespaces: []string{"kube-system"},
				},
			},
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	// worker should have feature disabled even if it is enabled in the config
	specMD := resource.NewMetadata(config.NamespaceName, kubeaccess.ConfigType, kubeaccess.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(specMD)))
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
