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
			ClusterID:     "cluster1",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
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
				suite.Assert().True(spec.RegistryServiceEnabled)
				suite.Assert().Equal("discovery.talos.dev:443", spec.ServiceEndpoint)
				suite.Assert().Equal("cluster1", spec.ServiceClusterID)
				suite.Assert().Equal(
					[]byte("\x90\x24\x2c\x2a\xbe\x01\xdb\xc5\x54\x97\xba\xb0\xd6\xc5\x64\x4c\x33\x45\xf5\xf1\x47\xfb\xe5\x62\x22\xe2\xac\xb2\xcf\x82\xea\x47"),
					spec.ServiceEncryptionKey)

				return nil
			},
		),
	))
}

func (suite *ConfigSuite) TestReconcileConfigCustom() {
	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.ConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "cluster1",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: v1alpha1.ClusterDiscoveryConfig{
				DiscoveryEnabled: true,
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{
					RegistryKubernetes: v1alpha1.RegistryKubernetesConfig{
						RegistryDisabled: true,
					},
					RegistryService: v1alpha1.RegistryServiceConfig{
						RegistryEndpoint: "https://[2001:470:6d:30e:565d:e162:e2a0:cf5a]:3456/",
					},
				},
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
				suite.Assert().False(spec.RegistryKubernetesEnabled)
				suite.Assert().True(spec.RegistryServiceEnabled)
				suite.Assert().Equal("[2001:470:6d:30e:565d:e162:e2a0:cf5a]:3456", spec.ServiceEndpoint)

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
