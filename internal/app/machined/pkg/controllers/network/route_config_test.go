// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"log"
	"net/netip"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RouteConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *RouteConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.DeviceConfigController{}))
}

func (suite *RouteConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *RouteConfigSuite) assertRoutes(requiredIDs []string, check func(*network.RouteSpec, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *RouteConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.RouteConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp ip=10.3.5.7::10.3.5.1:255.255.255.0::eth4"),
			},
		),
	)

	suite.startRuntime()

	suite.assertRoutes(
		[]string{
			"cmdline/inet4/172.20.0.1//1024",
			"cmdline/inet4/10.3.5.1//1026",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
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
	)
}

func (suite *RouteConfigSuite) TestCmdlineNotReachable() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.RouteConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.255::eth1:::::"),
			},
		),
	)

	suite.startRuntime()

	suite.assertRoutes(
		[]string{
			"cmdline/inet4/172.20.0.1//1024",
			"cmdline/inet4//172.20.0.1/32/1024",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
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
	)
}

func (suite *RouteConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.RouteConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{
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
								DeviceIgnore:    pointer.To(true),
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

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.assertRoutes(
		[]string{
			"configuration/eth2/inet6/2001:470:6d:30e:8ed2:b60c:9d2f:803b//1024",
			"configuration/inet4/10.0.3.1/10.0.3.0/24/1024",
			"configuration/inet4/192.168.0.25/192.168.0.0/18/25",
			"configuration/inet4/192.244.0.1/192.244.0.0/24/1024",
			"configuration/inet4//169.254.254.254/32/1024",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
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
	)
}

func (suite *RouteConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestRouteConfigSuite(t *testing.T) {
	suite.Run(t, new(RouteConfigSuite))
}
