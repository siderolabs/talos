// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ResolverConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ResolverConfigSuite) TestDefaults() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ResolverConfigController{}))

	ctest.AssertResources(
		suite,
		[]string{
			"default/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]netip.Addr{
					netip.MustParseAddr(constants.DefaultPrimaryResolver),
					netip.MustParseAddr(constants.DefaultSecondaryResolver),
				}, r.TypedSpec().DNSServers,
			)
			asrt.Empty(r.TypedSpec().SearchDomains)
			asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *ResolverConfigSuite) TestWithHostnameStatus() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ResolverConfigController{}))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "irrelevant"
	hostnameStatus.TypedSpec().Domainname = "example.org"
	suite.Create(hostnameStatus)

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{}, //nolint:staticcheck // legacy config
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

	ctest.AssertResources(
		suite,
		[]string{
			"default/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]netip.Addr{
					netip.MustParseAddr(constants.DefaultPrimaryResolver),
					netip.MustParseAddr(constants.DefaultSecondaryResolver),
				}, r.TypedSpec().DNSServers,
			)
			asrt.Equal([]string{"example.org"}, r.TypedSpec().SearchDomains)
			asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	// make domain name empty
	hostnameStatus.TypedSpec().Domainname = ""
	suite.Update(hostnameStatus)

	ctest.AssertResources(
		suite,
		[]string{
			"default/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Empty(r.TypedSpec().SearchDomains)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	// bring back domain name, but disable via machine config
	hostnameStatus.TypedSpec().Domainname = "example.org"
	suite.Update(hostnameStatus)

	cfg.Container().RawV1Alpha1().MachineConfig.MachineNetwork.NetworkDisableSearchDomain = pointer.To(true) //nolint:staticcheck
	suite.Update(cfg)

	ctest.AssertResources(
		suite,
		[]string{
			"default/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Empty(r.TypedSpec().SearchDomains)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *ResolverConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.ResolverConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2:172.21.0.1:172.20.0.1:255.255.255.0:master1:eth1::10.0.0.1:10.0.0.2:10.0.0.1"),
			},
		),
	)

	ctest.AssertResources(
		suite,
		[]string{
			"cmdline/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]netip.Addr{
					netip.MustParseAddr("10.0.0.1"),
					netip.MustParseAddr("10.0.0.2"),
				}, r.TypedSpec().DNSServers,
			)
			asrt.Empty(r.TypedSpec().SearchDomains)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *ResolverConfigSuite) TestMachineConfigurationLegacy() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ResolverConfigController{}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NameServers: []string{"2.2.2.2", "3.3.3.3"},
						Searches:    []string{"example.com", "example.org"},
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

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]netip.Addr{
					netip.MustParseAddr("2.2.2.2"),
					netip.MustParseAddr("3.3.3.3"),
				}, r.TypedSpec().DNSServers,
			)

			asrt.Equal(
				[]string{"example.com", "example.org"},
				r.TypedSpec().SearchDomains,
			)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachineNetwork.NameServers = nil //nolint:staticcheck
		r.Container().RawV1Alpha1().MachineConfig.MachineNetwork.Searches = nil    //nolint:staticcheck

		return nil
	})

	ctest.AssertNoResource[*network.ResolverSpec](suite, "configuration/resolvers", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *ResolverConfigSuite) TestMachineConfigurationNewStyle() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ResolverConfigController{}))

	rc := networkcfg.NewResolverConfigV1Alpha1()
	rc.ResolverNameservers = []networkcfg.NameserverConfig{
		{
			Address: networkcfg.Addr{Addr: netip.MustParseAddr("2.2.2.2")},
		},
		{
			Address: networkcfg.Addr{Addr: netip.MustParseAddr("3.3.3.3")},
		},
	}
	rc.ResolverSearchDomains = networkcfg.SearchDomainsConfig{
		SearchDomains: []string{"example.com", "example.org"},
	}

	ctr, err := container.New(rc)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]netip.Addr{
					netip.MustParseAddr("2.2.2.2"),
					netip.MustParseAddr("3.3.3.3"),
				}, r.TypedSpec().DNSServers,
			)

			asrt.Equal(
				[]string{"example.com", "example.org"},
				r.TypedSpec().SearchDomains,
			)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	suite.Destroy(cfg)

	ctest.AssertNoResource[*network.ResolverSpec](suite, "configuration/resolvers", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func TestResolverConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ResolverConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
		},
	})
}
