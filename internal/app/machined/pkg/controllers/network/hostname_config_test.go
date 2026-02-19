// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HostnameConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *HostnameConfigSuite) TestNoDefaultWithoutMachineConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameConfigController{}))

	defaultAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID)
	defaultAddress.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("33.11.22.44/32")}

	suite.Create(defaultAddress)

	ctest.AssertNoResource[*network.HostnameSpec](suite, "default/hostname", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *HostnameConfigSuite) TestDefaultIPBasedHostname() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameConfigController{}))

	suite.Create(config.NewMachineConfig(container.NewV1Alpha1(&v1alpha1.Config{ConfigVersion: "v1alpha1"})))

	defaultAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID)
	defaultAddress.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("33.11.22.44/32")}
	suite.Create(defaultAddress)

	ctest.AssertResource(suite, "default/hostname", func(r *network.HostnameSpec, asrt *assert.Assertions) {
		asrt.Equal("talos-33-11-22-44", r.TypedSpec().Hostname)
		asrt.Equal("", r.TypedSpec().Domainname)
		asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *HostnameConfigSuite) TestDefaultStableHostname() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameConfigController{}))

	suite.Create(config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						StableHostname: new(true),
					},
				},
			},
		),
	))

	id := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	id.TypedSpec().NodeID = "fGdOI05hVrx3YMagLo0Bwxa2Nm9BAswWm8XLeEj0aS4"
	suite.Create(id)

	ctest.AssertResource(suite, "default/hostname", func(r *network.HostnameSpec, asrt *assert.Assertions) {
		asrt.Equal("talos-hwz-sw5", r.TypedSpec().Hostname)
		asrt.Equal("", r.TypedSpec().Domainname)
		asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *HostnameConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.HostnameConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2:172.21.0.1:172.20.0.1:255.255.255.0:master1.domain.tld:eth1::10.0.0.1:10.0.0.2:10.0.0.1"),
			},
		),
	)

	ctest.AssertResource(suite, "cmdline/hostname", func(r *network.HostnameSpec, asrt *assert.Assertions) {
		asrt.Equal("master1", r.TypedSpec().Hostname)
		asrt.Equal("domain.tld", r.TypedSpec().Domainname)
		asrt.Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *HostnameConfigSuite) TestLegacyMachineConfiguration() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameConfigController{}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkHostname: "foo",
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(
		suite, "configuration/hostname", func(r *network.HostnameSpec, asrt *assert.Assertions) {
			asrt.Equal("foo", r.TypedSpec().Hostname)
			asrt.Equal("", r.TypedSpec().Domainname)
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
		}, rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachineNetwork.NetworkHostname = strings.Repeat("a", 128) //nolint:staticcheck // using legacy field in the test

		return nil
	})
	suite.Require().NoError(err)

	ctest.AssertNoResource[*network.HostnameSpec](suite, "configuration/hostname", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *HostnameConfigSuite) TestMachineConfigurationStaticHostname() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameConfigController{}))

	hostnameCfg := networkcfg.NewHostnameConfigV1Alpha1()
	hostnameCfg.ConfigAuto = new(nethelpers.AutoHostnameKindOff)
	hostnameCfg.ConfigHostname = "my-hostname"

	ctr, err := container.New(hostnameCfg)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertResource(
		suite, "configuration/hostname", func(r *network.HostnameSpec, asrt *assert.Assertions) {
			asrt.Equal("my-hostname", r.TypedSpec().Hostname)
			asrt.Equal("", r.TypedSpec().Domainname)
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
		}, rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.AssertNoResource[*network.HostnameSpec](suite, "default/hostname", rtestutils.WithNamespace(network.ConfigNamespaceName))

	suite.Destroy(cfg)

	ctest.AssertNoResource[*network.HostnameSpec](suite, "configuration/hostname", rtestutils.WithNamespace(network.ConfigNamespaceName))
	ctest.AssertNoResource[*network.HostnameSpec](suite, "default/hostname", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *HostnameConfigSuite) TestMachineConfigurationDefaultStable() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.HostnameConfigController{}))

	hostnameCfg := networkcfg.NewHostnameConfigV1Alpha1()
	hostnameCfg.ConfigAuto = new(nethelpers.AutoHostnameKindStable)

	ctr, err := container.New(hostnameCfg)
	suite.Require().NoError(err)

	id := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	id.TypedSpec().NodeID = "fGdOI05hVrx3YMagLo0Bwxa2Nm9BAswWm8XLeEj0aS4"
	suite.Create(id)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertResource(suite, "default/hostname", func(r *network.HostnameSpec, asrt *assert.Assertions) {
		asrt.Equal("talos-hwz-sw5", r.TypedSpec().Hostname)
		asrt.Equal("", r.TypedSpec().Domainname)
		asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func TestHostnameConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &HostnameConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
