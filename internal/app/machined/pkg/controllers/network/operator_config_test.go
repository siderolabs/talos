// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *OperatorConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.DeviceConfigController{}))
}

func (suite *OperatorConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *OperatorConfigSuite) assertOperators(requiredIDs []string, check func(*network.OperatorSpec, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *OperatorConfigSuite) assertNoOperators(unexpectedIDs []string) error {
	unexpIDs := make(map[string]struct{}, len(unexpectedIDs))

	for _, id := range unexpectedIDs {
		unexpIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, unexpected := unexpIDs[res.Metadata().ID()]
		if unexpected {
			return retry.ExpectedErrorf("unexpected ID %q", res.Metadata().ID())
		}
	}

	return nil
}

func (suite *OperatorConfigSuite) TestDefaultDHCP() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.OperatorConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth2"),
			},
		),
	)

	suite.startRuntime()

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Require().NoError(suite.state.Create(suite.ctx, linkStatus))
	}

	suite.assertOperators(
		[]string{
			"default/dhcp4/eth0",
			"default/dhcp4/eth1",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)
			asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)

			switch r.Metadata().ID() {
			case "default/dhcp4/eth0":
				asrt.Equal("eth0", r.TypedSpec().LinkName)
			case "default/dhcp4/eth1":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
			}
		},
	)
}

func (suite *OperatorConfigSuite) TestDefaultDHCPCmdline() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.OperatorConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1::::: ip=eth3:dhcp"),
			},
		),
	)

	suite.startRuntime()

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Require().NoError(suite.state.Create(suite.ctx, linkStatus))
	}

	suite.assertOperators(
		[]string{
			"default/dhcp4/eth0",
			"default/dhcp4/eth2",
			"cmdline/dhcp4/eth3",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)
			asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)

			switch r.Metadata().ID() {
			case "default/dhcp4/eth0":
				asrt.Equal("eth0", r.TypedSpec().LinkName)
			case "default/dhcp4/eth2":
				asrt.Equal("eth2", r.TypedSpec().LinkName)
			case "cmdline/dhcp4/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
			}
		},
	)

	// remove link
	suite.Require().NoError(
		suite.state.Destroy(
			suite.ctx,
			resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "eth2", resource.VersionUndefined),
		),
	)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoOperators(
					[]string{
						"default/dhcp4/eth2",
					},
				)
			},
		),
	)
}

func (suite *OperatorConfigSuite) TestMachineConfigurationDHCP4() {
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.OperatorConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth5"),
			},
		),
	)
	// add LinkConfig controller to produce link specs based on machine configuration
	suite.Require().NoError(
		suite.runtime.RegisterController(
			&netctrl.LinkConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth5"),
			},
		),
	)

	suite.startRuntime()

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Require().NoError(suite.state.Create(suite.ctx, linkStatus))
	}

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
								DeviceInterface: "eth0",
							},
							{
								DeviceInterface: "eth1",
								DeviceDHCP:      pointer.To(true),
							},
							{
								DeviceIgnore:    pointer.To(true),
								DeviceInterface: "eth2",
								DeviceDHCP:      pointer.To(true),
							},
							{
								DeviceInterface: "eth3",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv4:        pointer.To(true),
									DHCPRouteMetric: 256,
								},
							},
							{
								DeviceInterface: "eth4",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID:   25,
										VlanDHCP: pointer.To(true),
									},
									{
										VlanID: 26,
									},
									{
										VlanID: 27,
										VlanDHCPOptions: &v1alpha1.DHCPOptions{
											DHCPRouteMetric: 256,
										},
									},
								},
							},
							{
								DeviceInterface: "eth5",
								DeviceDHCP:      pointer.To(true),
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

	suite.assertOperators(
		[]string{
			"configuration/dhcp4/eth1",
			"configuration/dhcp4/eth3",
			"configuration/dhcp4/eth4.25",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/dhcp4/eth1":
				asrt.Equal("eth1", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
				asrt.EqualValues(256, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth4.25":
				asrt.Equal("eth4.25", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth4.26":
				asrt.Equal("eth4.26", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
			case "configuration/dhcp4/eth4.27":
				asrt.Equal("eth4.27", r.TypedSpec().LinkName)
				asrt.EqualValues(256, r.TypedSpec().DHCP4.RouteMetric)
			}
		},
	)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoOperators(
					[]string{
						"configuration/dhcp4/eth0",
						"default/dhcp4/eth0",
						"configuration/dhcp4/eth2",
						"default/dhcp4/eth2",
						"configuration/dhcp4/eth4.26",
					},
				)
			},
		),
	)
}

func (suite *OperatorConfigSuite) TestMachineConfigurationDHCP6() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorConfigController{}))

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
								DeviceInterface: "eth1",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv4: pointer.To(true),
								},
							},
							{
								DeviceInterface: "eth2",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv6: pointer.To(true),
								},
							},
							{
								DeviceInterface: "eth3",
								DeviceDHCP:      pointer.To(true),
								DeviceDHCPOptions: &v1alpha1.DHCPOptions{
									DHCPIPv6:        pointer.To(true),
									DHCPRouteMetric: 512,
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

	suite.assertOperators(
		[]string{
			"configuration/dhcp6/eth2",
			"configuration/dhcp6/eth3",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(network.OperatorDHCP6, r.TypedSpec().Operator)
			asrt.True(r.TypedSpec().RequireUp)

			switch r.Metadata().ID() {
			case "configuration/dhcp6/eth2":
				asrt.Equal("eth2", r.TypedSpec().LinkName)
				asrt.EqualValues(network.DefaultRouteMetric, r.TypedSpec().DHCP6.RouteMetric)
			case "configuration/dhcp6/eth3":
				asrt.Equal("eth3", r.TypedSpec().LinkName)
				asrt.EqualValues(512, r.TypedSpec().DHCP6.RouteMetric)
			}
		},
	)

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoOperators(
					[]string{
						"configuration/dhcp6/eth1",
					},
				)
			},
		),
	)
}

func (suite *OperatorConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestOperatorConfigSuite(t *testing.T) {
	suite.Run(t, new(OperatorConfigSuite))
}
