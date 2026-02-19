// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package cluster_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type ConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ConfigSuite) TestReconcileConfig() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "cluster1",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
				DiscoveryEnabled: new(true),
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.DiscoveryEnabled)
			asrt.True(spec.RegistryKubernetesEnabled)
			asrt.True(spec.RegistryServiceEnabled)
			asrt.Equal("discovery.talos.dev:443", spec.ServiceEndpoint)
			asrt.False(spec.ServiceEndpointInsecure)
			asrt.Equal("cluster1", spec.ServiceClusterID)
			asrt.Equal(
				[]byte("\x90\x24\x2c\x2a\xbe\x01\xdb\xc5\x54\x97\xba\xb0\xd6\xc5\x64\x4c\x33\x45\xf5\xf1\x47\xfb\xe5\x62\x22\xe2\xac\xb2\xcf\x82\xea\x47"),
				spec.ServiceEncryptionKey,
			)
		})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), cfg.Metadata()))

	rtestutils.AssertNoResource[*cluster.Config](suite.Ctx(), suite.T(), suite.State(), cluster.ConfigID)
}

func (suite *ConfigSuite) TestReconcileConfigCustom() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "cluster1",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
				DiscoveryEnabled: new(true),
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{
					RegistryKubernetes: v1alpha1.RegistryKubernetesConfig{
						RegistryDisabled: new(true),
					},
					RegistryService: v1alpha1.RegistryServiceConfig{
						RegistryEndpoint: "https://[2001:470:6d:30e:565d:e162:e2a0:cf5a]:3456/",
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.DiscoveryEnabled)
			asrt.False(spec.RegistryKubernetesEnabled)
			asrt.True(spec.RegistryServiceEnabled)
			asrt.Equal("[2001:470:6d:30e:565d:e162:e2a0:cf5a]:3456", spec.ServiceEndpoint)
			asrt.False(spec.ServiceEndpointInsecure)
		},
	)
}

func (suite *ConfigSuite) TestReconcileConfigCustomInsecure() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "cluster1",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{
				DiscoveryEnabled: new(true),
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{
					RegistryKubernetes: v1alpha1.RegistryKubernetesConfig{
						RegistryDisabled: new(true),
					},
					RegistryService: v1alpha1.RegistryServiceConfig{
						RegistryEndpoint: "http://localhost:3000",
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.DiscoveryEnabled)
			asrt.False(spec.RegistryKubernetesEnabled)
			asrt.True(spec.RegistryServiceEnabled)
			asrt.Equal("localhost:3000", spec.ServiceEndpoint)
			asrt.True(spec.ServiceEndpointInsecure)
		},
	)
}

func (suite *ConfigSuite) TestReconcileDisabled() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.False(spec.DiscoveryEnabled)
			asrt.False(spec.RegistryKubernetesEnabled)
		},
	)
}

func (suite *ConfigSuite) TestReconcilePartial() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.False(spec.DiscoveryEnabled)
			asrt.False(spec.RegistryKubernetesEnabled)
		},
	)

	newCfg := config.NewMachineConfig(must(container.New()))
	newCfg.Metadata().SetVersion(cfg.Metadata().Version())

	suite.Require().NoError(suite.State().Update(suite.Ctx(), newCfg))

	rtestutils.AssertNoResource[*cluster.Config](suite.Ctx(), suite.T(), suite.State(), cluster.ConfigID)
}

func TestConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(clusterctrl.NewConfigController()))
			},
		},
	})
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
