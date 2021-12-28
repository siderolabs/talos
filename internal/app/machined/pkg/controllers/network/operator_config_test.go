// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type OperatorConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *OperatorConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)
}

func (suite *OperatorConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *OperatorConfigSuite) assertOperators(requiredIDs []string, check func(*network.OperatorSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.OperatorSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *OperatorConfigSuite) assertNoOperators(unexpectedIDs []string) error {
	unexpIDs := make(map[string]struct{}, len(unexpectedIDs))

	for _, id := range unexpectedIDs {
		unexpIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
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
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorConfigController{
		Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth2"),
	}))

	suite.startRuntime()

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Require().NoError(suite.state.Create(suite.ctx, linkStatus))
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertOperators([]string{
				"default/dhcp4/eth0",
				"default/dhcp4/eth1",
			}, func(r *network.OperatorSpec) error {
				suite.Assert().Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
				suite.Assert().True(r.TypedSpec().RequireUp)
				suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)

				switch r.Metadata().ID() {
				case "default/dhcp4/eth0":
					suite.Assert().Equal("eth0", r.TypedSpec().LinkName)
				case "default/dhcp4/eth1":
					suite.Assert().Equal("eth1", r.TypedSpec().LinkName)
				}

				return nil
			})
		}))
}

func (suite *OperatorConfigSuite) TestDefaultDHCPCmdline() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorConfigController{
		Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1:::::"),
	}))

	suite.startRuntime()

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Require().NoError(suite.state.Create(suite.ctx, linkStatus))
	}

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertOperators([]string{
				"default/dhcp4/eth0",
				"default/dhcp4/eth2",
			}, func(r *network.OperatorSpec) error {
				suite.Assert().Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
				suite.Assert().True(r.TypedSpec().RequireUp)
				suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)

				switch r.Metadata().ID() {
				case "default/dhcp4/eth0":
					suite.Assert().Equal("eth0", r.TypedSpec().LinkName)
				case "default/dhcp4/eth2":
					suite.Assert().Equal("eth2", r.TypedSpec().LinkName)
				}

				return nil
			})
		}))

	// remove link
	suite.Require().NoError(suite.state.Destroy(suite.ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "eth2", resource.VersionUndefined)))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoOperators([]string{
				"default/dhcp4/eth2",
			})
		}))
}

func (suite *OperatorConfigSuite) TestMachineConfigurationDHCP4() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorConfigController{
		Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth5"),
	}))
	// add LinkConfig controller to produce link specs based on machine configuration
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkConfigController{
		Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth5"),
	}))

	suite.startRuntime()

	for _, link := range []string{"eth0", "eth1", "eth2"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		suite.Require().NoError(suite.state.Create(suite.ctx, linkStatus))
	}

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
					},
					{
						DeviceInterface: "eth1",
						DeviceDHCP:      true,
					},
					{
						DeviceIgnore:    true,
						DeviceInterface: "eth2",
						DeviceDHCP:      true,
					},
					{
						DeviceInterface: "eth3",
						DeviceDHCP:      true,
						DeviceDHCPOptions: &v1alpha1.DHCPOptions{
							DHCPIPv4:        pointer.ToBool(true),
							DHCPRouteMetric: 256,
						},
					},
					{
						DeviceInterface: "eth4",
						DeviceVlans: []*v1alpha1.Vlan{
							{
								VlanID:   25,
								VlanDHCP: true,
							},
							{
								VlanID: 26,
							},
						},
					},
					{
						DeviceInterface: "eth5",
						DeviceDHCP:      true,
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertOperators([]string{
				"configuration/dhcp4/eth1",
				"configuration/dhcp4/eth3",
				"configuration/dhcp4/eth4.25",
			}, func(r *network.OperatorSpec) error {
				suite.Assert().Equal(network.OperatorDHCP4, r.TypedSpec().Operator)
				suite.Assert().True(r.TypedSpec().RequireUp)

				switch r.Metadata().ID() {
				case "configuration/dhcp4/eth1":
					suite.Assert().Equal("eth1", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
				case "configuration/dhcp4/eth3":
					suite.Assert().Equal("eth3", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(256, r.TypedSpec().DHCP4.RouteMetric)
				case "configuration/dhcp4/eth4.25":
					suite.Assert().Equal("eth4.25", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.TypedSpec().DHCP4.RouteMetric)
				}

				return nil
			})
		}))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoOperators([]string{
				"configuration/dhcp4/eth0",
				"default/dhcp4/eth0",
				"configuration/dhcp4/eth2",
				"default/dhcp4/eth2",
				"configuration/dhcp4/eth4.26",
			})
		}))
}

func (suite *OperatorConfigSuite) TestMachineConfigurationDHCP6() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth1",
						DeviceDHCP:      true,
						DeviceDHCPOptions: &v1alpha1.DHCPOptions{
							DHCPIPv4: pointer.ToBool(true),
						},
					},
					{
						DeviceInterface: "eth2",
						DeviceDHCP:      true,
						DeviceDHCPOptions: &v1alpha1.DHCPOptions{
							DHCPIPv6: pointer.ToBool(true),
						},
					},
					{
						DeviceInterface: "eth3",
						DeviceDHCP:      true,
						DeviceDHCPOptions: &v1alpha1.DHCPOptions{
							DHCPIPv6:        pointer.ToBool(true),
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertOperators([]string{
				"configuration/dhcp6/eth2",
				"configuration/dhcp6/eth3",
			}, func(r *network.OperatorSpec) error {
				suite.Assert().Equal(network.OperatorDHCP6, r.TypedSpec().Operator)
				suite.Assert().True(r.TypedSpec().RequireUp)

				switch r.Metadata().ID() {
				case "configuration/dhcp6/eth2":
					suite.Assert().Equal("eth2", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(netctrl.DefaultRouteMetric, r.TypedSpec().DHCP6.RouteMetric)
				case "configuration/dhcp6/eth3":
					suite.Assert().Equal("eth3", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(512, r.TypedSpec().DHCP6.RouteMetric)
				}

				return nil
			})
		}))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoOperators([]string{
				"configuration/dhcp6/eth1",
			})
		}))
}

func (suite *OperatorConfigSuite) TestMachineConfigurationVIP() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.OperatorConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth1",
						DeviceDHCP:      true,
						DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
							SharedIP: "2.3.4.5",
						},
					},
					{
						DeviceInterface: "eth2",
						DeviceDHCP:      true,
						DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
							SharedIP: "fd7a:115c:a1e0:ab12:4843:cd96:6277:2302",
						},
					},
					{
						DeviceInterface: "eth3",
						DeviceDHCP:      true,
						DeviceVlans: []*v1alpha1.Vlan{
							{
								VlanID: 26,
								VlanVIP: &v1alpha1.DeviceVIPConfig{
									SharedIP: "5.5.4.4",
								},
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
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertOperators([]string{
				"configuration/vip/eth1",
				"configuration/vip/eth2",
				"configuration/vip/eth3.26",
			}, func(r *network.OperatorSpec) error {
				suite.Assert().Equal(network.OperatorVIP, r.TypedSpec().Operator)
				suite.Assert().True(r.TypedSpec().RequireUp)

				switch r.Metadata().ID() {
				case "configuration/vip/eth1":
					suite.Assert().Equal("eth1", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(netaddr.MustParseIP("2.3.4.5"), r.TypedSpec().VIP.IP)
				case "configuration/vip/eth2":
					suite.Assert().Equal("eth2", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(netaddr.MustParseIP("fd7a:115c:a1e0:ab12:4843:cd96:6277:2302"), r.TypedSpec().VIP.IP)
				case "configuration/vip/eth3.26":
					suite.Assert().Equal("eth3.26", r.TypedSpec().LinkName)
					suite.Assert().EqualValues(netaddr.MustParseIP("5.5.4.4"), r.TypedSpec().VIP.IP)
				}

				return nil
			})
		}))
}

func (suite *OperatorConfigSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	err := suite.state.Create(context.Background(), config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
	}))
	if state.IsConflictError(err) {
		err = suite.state.Destroy(context.Background(), config.NewMachineConfig(nil).Metadata())
	}

	suite.Require().NoError(err)

	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewLinkStatus(network.ConfigNamespaceName, "bar")))
}

func TestOperatorConfigSuite(t *testing.T) {
	suite.Run(t, new(OperatorConfigSuite))
}
