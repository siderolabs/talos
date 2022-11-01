// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	kubespanctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubespan"
)

type ConfigSuite struct {
	KubeSpanSuite
}

func (suite *ConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&kubespanctrl.ConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{
					KubeSpanEnabled: pointer.To(true),
				},
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=",
			ClusterSecret: "I+1In7fLnpcRIjUmEoeugZnSyFoTF6MztLxICL5Yu0s=",
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(config.NamespaceName, kubespan.ConfigType, kubespan.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*kubespan.Config).TypedSpec()

				suite.Assert().True(spec.Enabled)
				suite.Assert().Equal("8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=", spec.ClusterID)
				suite.Assert().Equal("I+1In7fLnpcRIjUmEoeugZnSyFoTF6MztLxICL5Yu0s=", spec.SharedSecret)
				suite.Assert().True(spec.ForceRouting)
				suite.Assert().False(spec.AdvertiseKubernetesNetworks)

				return nil
			},
		),
	))
}

func (suite *ConfigSuite) TestReconcileDisabled() {
	suite.Require().NoError(suite.runtime.RegisterController(&kubespanctrl.ConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(config.NamespaceName, kubespan.ConfigType, kubespan.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*kubespan.Config).TypedSpec()

				suite.Assert().False(spec.Enabled)

				return nil
			},
		),
	))
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
