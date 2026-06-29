// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package cluster_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	clustertypes "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
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
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy configuration
				DiscoveryEnabled: new(true),
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.RegistryKubernetesEnabled)
			asrt.Equal([]cluster.ServiceEndpoint{{Name: "legacy", Endpoint: "discovery.talos.dev:443", Insecure: false}}, spec.ServiceEndpoints)
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
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy configuration
				DiscoveryEnabled: new(true),
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // legacy configuration
					RegistryKubernetes: v1alpha1.RegistryKubernetesConfig{ //nolint:staticcheck // legacy configuration
						RegistryDisabled: new(true),
					},
					RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // legacy configuration
						RegistryEndpoint: "https://[2001:470:6d:30e:565d:e162:e2a0:cf5a]:3456/",
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.False(spec.RegistryKubernetesEnabled)
			asrt.Equal([]cluster.ServiceEndpoint{{Name: "legacy", Endpoint: "[2001:470:6d:30e:565d:e162:e2a0:cf5a]:3456", Insecure: false}}, spec.ServiceEndpoints)
		},
	)
}

func (suite *ConfigSuite) TestReconcileConfigCustomInsecure() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "cluster1",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy configuration
				DiscoveryEnabled: new(true),
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // legacy configuration
					RegistryKubernetes: v1alpha1.RegistryKubernetesConfig{ //nolint:staticcheck // legacy configuration
						RegistryDisabled: new(true),
					},
					RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // legacy configuration
						RegistryEndpoint: "http://localhost:3000",
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.False(spec.RegistryKubernetesEnabled)
			asrt.Equal([]cluster.ServiceEndpoint{{Name: "legacy", Endpoint: "localhost:3000", Insecure: true}}, spec.ServiceEndpoints)
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

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Empty(spec.ServiceEndpoints)
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

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Empty(spec.ServiceEndpoints)
			asrt.False(spec.RegistryKubernetesEnabled)
		},
	)

	newCfg := config.NewMachineConfig(must(container.New()))
	newCfg.Metadata().SetVersion(cfg.Metadata().Version())

	suite.Require().NoError(suite.State().Update(suite.Ctx(), newCfg))

	rtestutils.AssertNoResource[*cluster.Config](suite.Ctx(), suite.T(), suite.State(), cluster.ConfigID)
}

func (suite *ConfigSuite) TestLegacyFieldsWithDiscoveryService() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "test-cluster",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy config
				DiscoveryEnabled: new(true),
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.RegistryServiceEnabled, "RegistryServiceEnabled should be true when discovery is configured")                                            //nolint:staticcheck // legacy config
			asrt.NotEmpty(spec.ServiceEndpoint, "ServiceEndpoint should not be empty when discovery is configured")                                                 //nolint:staticcheck // legacy config
			asrt.Equal(spec.ServiceEndpoints[0].Endpoint, spec.ServiceEndpoint, "legacy ServiceEndpoint should match first ServiceEndpoints entry")                 //nolint:staticcheck // legacy config
			asrt.Equal(spec.ServiceEndpoints[0].Insecure, spec.ServiceEndpointInsecure, "legacy ServiceEndpointInsecure should match first ServiceEndpoints entry") //nolint:staticcheck // legacy config
		},
	)
}

func (suite *ConfigSuite) TestLegacyFieldsWithoutDiscoveryService() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.False(spec.RegistryServiceEnabled, "RegistryServiceEnabled should be false when discovery is not configured")   //nolint:staticcheck // legacy config
			asrt.Empty(spec.ServiceEndpoint, "ServiceEndpoint should be empty when discovery is not configured")                 //nolint:staticcheck // legacy config
			asrt.False(spec.ServiceEndpointInsecure, "ServiceEndpointInsecure should be false when discovery is not configured") //nolint:staticcheck // legacy config
			asrt.Nil(spec.ServiceEndpoints, "ServiceEndpoints should be nil when discovery is not configured")
		},
	)
}

func (suite *ConfigSuite) TestLegacyFieldsInsecureEndpoint() {
	cfg := config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterID:     "test-cluster",
			ClusterSecret: "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc=",
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy config
				DiscoveryEnabled: new(true),
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // legacy config
					RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // legacy config
						RegistryEndpoint: "http://insecure.example.com:3000",
					},
				},
			},
		},
	}))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.True(spec.RegistryServiceEnabled, "RegistryServiceEnabled should be true")                               //nolint:staticcheck // legacy config
			asrt.True(spec.ServiceEndpointInsecure, "ServiceEndpointInsecure should be true for http endpoint")           //nolint:staticcheck // legacy config
			asrt.Equal("insecure.example.com:3000", spec.ServiceEndpoint, "ServiceEndpoint should be normalized address") //nolint:staticcheck // legacy config
		},
	)
}

// TestReconcileMultidocIdentity verifies the multi-doc DiscoveryIdentityConfig path produces the same
// cluster identity (ServiceClusterID/ServiceEncryptionKey) as the legacy .cluster.id/.cluster.secret fields
// asserted in TestReconcileConfig.
func (suite *ConfigSuite) TestReconcileMultidocIdentity() {
	endpointURL, err := url.Parse("https://discovery.talos.dev/")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(must(container.New(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			ClusterConfig: &v1alpha1.ClusterConfig{},
		},
		clustertypes.NewDiscoveryServiceConfigV1Alpha1("default", endpointURL),
		clustertypes.NewDiscoveryIdentityConfigV1Alpha1("cluster1", "kCQsKr4B28VUl7qw1sVkTDNF9fFH++ViIuKsss+C6kc="),
	)))

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{cluster.ConfigID},
		func(res *cluster.Config, asrt *assert.Assertions) {
			spec := res.TypedSpec()

			asrt.Equal([]cluster.ServiceEndpoint{{Name: "default", Endpoint: "discovery.talos.dev:443", Insecure: false}}, spec.ServiceEndpoints)
			asrt.Equal("cluster1", spec.ServiceClusterID)
			asrt.Equal(
				[]byte("\x90\x24\x2c\x2a\xbe\x01\xdb\xc5\x54\x97\xba\xb0\xd6\xc5\x64\x4c\x33\x45\xf5\xf1\x47\xfb\xe5\x62\x22\xe2\xac\xb2\xcf\x82\xea\x47"),
				spec.ServiceEncryptionKey,
			)
		})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), cfg.Metadata()))

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
