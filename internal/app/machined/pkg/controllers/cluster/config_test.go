// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package cluster_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	clusterctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/resources/cluster"
	"github.com/talos-systems/talos/pkg/resources/config"
)

type ConfigSuite struct {
	ClusterSuite
}

func (suite *ConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.ConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterDiscoveryConfig: v1alpha1.ClusterDiscoveryConfig{
				DiscoveryEnabled: true,
			},
		},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(config.NamespaceName, cluster.ConfigType, cluster.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*cluster.Config).TypedSpec()

				suite.Assert().True(spec.DiscoveryEnabled)
				suite.Assert().True(spec.RegistryKubernetesEnabled)

				return nil
			},
		),
	))
}

func (suite *ConfigSuite) TestReconcileDisabled() {
	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.ConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(config.NamespaceName, cluster.ConfigType, cluster.ConfigID, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) error {
				spec := res.(*cluster.Config).TypedSpec()

				suite.Assert().False(spec.DiscoveryEnabled)
				suite.Assert().False(spec.RegistryKubernetesEnabled)

				return nil
			},
		),
	))
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}
