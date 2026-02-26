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
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RouteConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *RouteConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.RouteConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp ip=10.3.5.7::10.3.5.1:255.255.255.0::eth4"),
			},
		),
	)

	ctest.AssertResources(
		suite,
		[]string{
			"cmdline/inet4/172.20.0.1//1024",
			"cmdline/inet4/10.3.5.1//1026",
		},
		func(r *network.RouteSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)
			asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)

			switch r.Metadata().ID() {
			case "cmdline/inet4/172.20.0.1//1024":
				asrt.Equal("eth1", r.TypedSpec().OutLinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
			case "cmdline/inet4/10.3.5.1//1025":
				asrt.Equal("eth4", r.TypedSpec().OutLinkName)
				asrt.EqualValues(network.DefaultRouteMetric+2, r.TypedSpec().Priority)
			}
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *RouteConfigSuite) TestCmdlineNotReachable() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.RouteConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.255::eth1:::::"),
			},
		),
	)

	ctest.AssertResources(
		suite,
		[]string{
			"cmdline/inet4/172.20.0.1//1024",
			"cmdline/inet4//172.20.0.1/32/1024",
		},
		func(r *network.RouteSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)
			asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)

			switch r.Metadata().ID() {
			case "cmdline/inet4/172.20.0.1//1024":
				asrt.Equal("eth1", r.TypedSpec().OutLinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
			case "cmdline/inet4//172.20.0.1/32/1024":
				asrt.Equal("eth1", r.TypedSpec().OutLinkName)
				asrt.Equal(netip.Addr{}, r.TypedSpec().Gateway)
				asrt.Equal(netip.MustParsePrefix("172.20.0.1/32"), r.TypedSpec().Destination)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
			}
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *RouteConfigSuite) TestMachineConfigurationLegacy() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RouteConfigController{}))
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DeviceConfigController{}))

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkInterfaces: []*v1alpha1.Device{
							{
								DeviceInterface: "eth3",
								DeviceAddresses: []string{"192.168.0.24/28"},
								DeviceRoutes: []*v1alpha1.Route{
									{
										RouteNetwork: "192.168.0.0/18",
										RouteGateway: "192.168.0.25",
										RouteMetric:  25,
									},
									{
										RouteNetwork: "169.254.254.254/32",
									},
								},
							},
							{
								DeviceIgnore:    new(true),
								DeviceInterface: "eth4",
								DeviceAddresses: []string{"192.168.0.24/28"},
								DeviceRoutes: []*v1alpha1.Route{
									{
										RouteNetwork: "192.168.0.0/18",
										RouteGateway: "192.168.0.26",
										RouteMetric:  25,
									},
								},
							},
							{
								DeviceInterface: "eth2",
								DeviceAddresses: []string{"2001:470:6d:30e:8ed2:b60c:9d2f:803a/64"},
								DeviceRoutes: []*v1alpha1.Route{
									{
										RouteGateway: "2001:470:6d:30e:8ed2:b60c:9d2f:803b",
									},
								},
							},
							{
								DeviceInterface: "eth0",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID: 24,
										VlanAddresses: []string{
											"10.0.0.1/8",
										},
										VlanRoutes: []*v1alpha1.Route{
											{
												RouteNetwork: "10.0.3.0/24",
												RouteGateway: "10.0.3.1",
											},
										},
									},
								},
							},
							{
								DeviceInterface: "eth1",
								DeviceRoutes: []*v1alpha1.Route{
									{
										RouteNetwork: "192.244.0.0/24",
										RouteGateway: "192.244.0.1",
										RouteSource:  "192.244.0.10",
									},
								},
							},
						},
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
			"configuration/eth2/inet6/2001:470:6d:30e:8ed2:b60c:9d2f:803b//1024",
			"configuration/inet4/10.0.3.1/10.0.3.0/24/1024",
			"configuration/inet4/192.168.0.25/192.168.0.0/18/25",
			"configuration/inet4/192.244.0.1/192.244.0.0/24/1024",
			"configuration/inet4//169.254.254.254/32/1024",
		},
		func(r *network.RouteSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "configuration/inet6/2001:470:6d:30e:8ed2:b60c:9d2f:803b//1024":
				asrt.Equal("eth2", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet6, r.TypedSpec().Family)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
			case "configuration/inet4/10.0.3.1/10.0.3.0/24/1024":
				asrt.Equal("eth0.24", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
			case "configuration/inet4/192.168.0.25/192.168.0.0/18/25":
				asrt.Equal("eth3", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
				asrt.EqualValues(25, r.TypedSpec().Priority)
			case "configuration/inet4/192.244.0.1/192.244.0.0/24/1024":
				asrt.Equal("eth1", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
				asrt.EqualValues(netip.MustParseAddr("192.244.0.10"), r.TypedSpec().Source)
			case "configuration/inet4//169.254.254.254/32/1024":
				asrt.Equal("eth3", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
				asrt.Equal(nethelpers.ScopeLink, r.TypedSpec().Scope)
				asrt.Equal("169.254.254.254/32", r.TypedSpec().Destination.String())
			}

			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *RouteConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RouteConfigController{}))

	lc1 := networkcfg.NewLinkConfigV1Alpha1("enp0s2")
	lc1.LinkRoutes = []networkcfg.RouteConfig{
		{
			RouteDestination: networkcfg.Prefix{Prefix: netip.MustParsePrefix("10.12.3.0/24")},
			RouteGateway:     networkcfg.Addr{Addr: netip.MustParseAddr("10.12.3.1")},
		},
	}

	lc2 := networkcfg.NewLinkConfigV1Alpha1("enp0s3")
	lc2.LinkRoutes = []networkcfg.RouteConfig{
		{
			RouteGateway: networkcfg.Addr{Addr: netip.MustParseAddr("2001:470:6d:30e:8ed2:b60c:9d2f:803b")},
			RouteMetric:  200,
		},
	}

	bc1 := networkcfg.NewBlackholeRouteConfigV1Alpha1("10.1.3.4/32")
	bc1.RouteMetric = 300

	ctr, err := container.New(lc1, lc2, bc1)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResources(
		suite,
		[]string{
			"configuration/enp0s3/inet6/2001:470:6d:30e:8ed2:b60c:9d2f:803b//200",
			"configuration/inet4/10.12.3.1/10.12.3.0/24/1024",
			"configuration/inet4//10.1.3.4/32/300",
		},
		func(r *network.RouteSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "configuration/enp0s3/inet6/2001:470:6d:30e:8ed2:b60c:9d2f:803b//200":
				asrt.Equal("enp0s3", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet6, r.TypedSpec().Family)
				asrt.EqualValues(200, r.TypedSpec().Priority)
			case "configuration/inet4/10.12.3.1/10.12.3.0/24/1024":
				asrt.Equal("enp0s2", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().Priority)
			case "configuration/inet4//10.1.3.4/32/300":
				asrt.Equal("lo", r.TypedSpec().OutLinkName)
				asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
				asrt.EqualValues(300, r.TypedSpec().Priority)
				asrt.Equal(nethelpers.TypeBlackhole, r.TypedSpec().Type)
			}

			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func TestRouteConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RouteConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
