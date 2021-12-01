// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,goconst
package network_test

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"testing"
	"time"

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

type LinkConfigSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *LinkConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)
}

func (suite *LinkConfigSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *LinkConfigSuite) assertLinks(requiredIDs []string, check func(*network.LinkSpec) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.LinkSpecType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.LinkSpec)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
}

func (suite *LinkConfigSuite) assertNoLinks(unexpectedIDs []string) error {
	unexpIDs := make(map[string]struct{}, len(unexpectedIDs))

	for _, id := range unexpectedIDs {
		unexpIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(suite.ctx, resource.NewMetadata(network.ConfigNamespaceName, network.LinkSpecType, "", resource.VersionUndefined))
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

func (suite *LinkConfigSuite) TestLoopback() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkConfigController{}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertLinks([]string{
				"default/lo",
			}, func(r *network.LinkSpec) error {
				suite.Assert().Equal("lo", r.TypedSpec().Name)
				suite.Assert().True(r.TypedSpec().Up)
				suite.Assert().False(r.TypedSpec().Logical)
				suite.Assert().Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)

				return nil
			})
		}))
}

func (suite *LinkConfigSuite) TestCmdline() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkConfigController{
		Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1:::::"),
	}))

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertLinks([]string{
				"cmdline/eth1",
			}, func(r *network.LinkSpec) error {
				suite.Assert().Equal("eth1", r.TypedSpec().Name)
				suite.Assert().True(r.TypedSpec().Up)
				suite.Assert().False(r.TypedSpec().Logical)
				suite.Assert().Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)

				return nil
			})
		}))
}

func (suite *LinkConfigSuite) TestMachineConfiguration() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkConfigController{}))

	suite.startRuntime()

	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineNetwork: &v1alpha1.NetworkConfig{
				NetworkInterfaces: []*v1alpha1.Device{
					{
						DeviceInterface: "eth0",
						DeviceVlans: []*v1alpha1.Vlan{
							{
								VlanID:  24,
								VlanMTU: 1000,
								VlanAddresses: []string{
									"10.0.0.1/8",
								},
							},
							{
								VlanID: 48,
								VlanAddresses: []string{
									"10.0.0.2/8",
								},
							},
						},
					},
					{
						DeviceInterface: "eth1",
						DeviceAddresses: []string{"192.168.0.24/28"},
					},
					{
						DeviceInterface: "eth1",
						DeviceMTU:       9001,
					},
					{
						DeviceIgnore:    true,
						DeviceInterface: "eth2",
						DeviceAddresses: []string{"192.168.0.24/28"},
					},
					{
						DeviceInterface: "eth2",
					},
					{
						DeviceInterface: "bond0",
						DeviceBond: &v1alpha1.Bond{
							BondInterfaces: []string{"eth2", "eth3"},
							BondMode:       "balance-xor",
						},
					},
					{
						DeviceInterface: "dummy0",
						DeviceDummy:     true,
					},
					{
						DeviceInterface: "wireguard0",
						DeviceWireguardConfig: &v1alpha1.DeviceWireguardConfig{
							WireguardPrivateKey: "ABC",
							WireguardPeers: []*v1alpha1.DeviceWireguardPeer{
								{
									WireguardPublicKey: "DEF",
									WireguardEndpoint:  "10.0.0.1:3000",
									WireguardAllowedIPs: []string{
										"10.2.3.0/24",
										"10.2.4.0/24",
									},
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
			return suite.assertLinks([]string{
				"configuration/eth0",
				"configuration/eth0.24",
				"configuration/eth0.48",
				"configuration/eth1",
				"configuration/eth2",
				"configuration/eth3",
				"configuration/bond0",
				"configuration/dummy0",
				"configuration/wireguard0",
			}, func(r *network.LinkSpec) error {
				suite.Assert().Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

				switch r.TypedSpec().Name {
				case "eth0", "eth1":
					suite.Assert().True(r.TypedSpec().Up)
					suite.Assert().False(r.TypedSpec().Logical)

					if r.TypedSpec().Name == "eth0" {
						suite.Assert().EqualValues(0, r.TypedSpec().MTU)
					} else {
						suite.Assert().EqualValues(9001, r.TypedSpec().MTU)
					}
				case "eth0.24", "eth0.48":
					suite.Assert().True(r.TypedSpec().Up)
					suite.Assert().True(r.TypedSpec().Logical)
					suite.Assert().Equal(nethelpers.LinkEther, r.TypedSpec().Type)
					suite.Assert().Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
					suite.Assert().Equal("eth0", r.TypedSpec().ParentName)
					suite.Assert().Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)

					if r.TypedSpec().Name == "eth0.24" {
						suite.Assert().EqualValues(24, r.TypedSpec().VLAN.VID)
						suite.Assert().EqualValues(1000, r.TypedSpec().MTU)
					} else {
						suite.Assert().EqualValues(48, r.TypedSpec().VLAN.VID)
						suite.Assert().EqualValues(0, r.TypedSpec().MTU)
					}
				case "eth2", "eth3":
					suite.Assert().True(r.TypedSpec().Up)
					suite.Assert().False(r.TypedSpec().Logical)
					suite.Assert().Equal("bond0", r.TypedSpec().MasterName)
				case "bond0":
					suite.Assert().True(r.TypedSpec().Up)
					suite.Assert().True(r.TypedSpec().Logical)
					suite.Assert().Equal(nethelpers.LinkEther, r.TypedSpec().Type)
					suite.Assert().Equal(network.LinkKindBond, r.TypedSpec().Kind)
					suite.Assert().Equal(nethelpers.BondModeXOR, r.TypedSpec().BondMaster.Mode)
					suite.Assert().True(r.TypedSpec().BondMaster.UseCarrier)
				case "wireguard0":
					suite.Assert().True(r.TypedSpec().Up)
					suite.Assert().True(r.TypedSpec().Logical)
					suite.Assert().Equal(nethelpers.LinkNone, r.TypedSpec().Type)
					suite.Assert().Equal(network.LinkKindWireguard, r.TypedSpec().Kind)
					suite.Assert().Equal(network.WireguardSpec{
						PrivateKey: "ABC",
						Peers: []network.WireguardPeer{
							{
								PublicKey: "DEF",
								Endpoint:  "10.0.0.1:3000",
								AllowedIPs: []netaddr.IPPrefix{
									netaddr.MustParseIPPrefix("10.2.3.0/24"),
									netaddr.MustParseIPPrefix("10.2.4.0/24"),
								},
							},
						},
					}, r.TypedSpec().Wireguard)
				}

				return nil
			})
		}))
}

func (suite *LinkConfigSuite) TestDefaultUp() {
	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.LinkConfigController{
		Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth2"),
	}))

	for _, link := range []string{"eth0", "eth1", "eth2", "eth3", "eth4"} {
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
						DeviceVlans: []*v1alpha1.Vlan{
							{
								VlanID: 24,
								VlanAddresses: []string{
									"10.0.0.1/8",
								},
							},
							{
								VlanID: 48,
								VlanAddresses: []string{
									"10.0.0.2/8",
								},
							},
						},
					},
					{
						DeviceInterface: "bond0",
						DeviceBond: &v1alpha1.Bond{
							BondInterfaces: []string{
								"eth3",
								"eth4",
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

	suite.startRuntime()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertLinks([]string{
				"default/eth1",
			}, func(r *network.LinkSpec) error {
				suite.Assert().Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
				suite.Assert().True(r.TypedSpec().Up)

				return nil
			})
		}))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertNoLinks([]string{
				"default/eth0",
				"default/eth2",
				"default/eth3",
				"default/eth4",
			})
		}))
}

func (suite *LinkConfigSuite) TearDownTest() {
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

	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewLinkStatus(network.NamespaceName, "bar")))
}

func TestLinkConfigSuite(t *testing.T) {
	suite.Run(t, new(LinkConfigSuite))
}
