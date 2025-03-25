// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst
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
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type LinkConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *LinkConfigSuite) assertLinks(requiredIDs []string, check func(*network.LinkSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *LinkConfigSuite) assertNoLinks(unexpectedIDs []string) {
	for _, id := range unexpectedIDs {
		ctest.AssertNoResource[*network.LinkSpec](suite, id, rtestutils.WithNamespace(network.ConfigNamespaceName))
	}
}

func (suite *LinkConfigSuite) TestLoopback() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

	suite.assertLinks(
		[]string{
			"default/lo",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal("lo", r.TypedSpec().Name)
			asrt.True(r.TypedSpec().Up)
			asrt.False(r.TypedSpec().Logical)
			asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
		},
	)
}

func (suite *LinkConfigSuite) TestCmdline() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.LinkConfigController{
				Cmdline: procfs.NewCmdline("ip=172.20.0.2::172.20.0.1:255.255.255.0::eth1:::::"),
			},
		),
	)

	suite.assertLinks(
		[]string{
			"cmdline/eth1",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal("eth1", r.TypedSpec().Name)
			asrt.True(r.TypedSpec().Up)
			asrt.False(r.TypedSpec().Logical)
			asrt.Equal(network.ConfigCmdline, r.TypedSpec().ConfigLayer)
		},
	)
}

//nolint:gocyclo
func (suite *LinkConfigSuite) TestMachineConfiguration() {
	const kernelDriver = "somekerneldriver"

	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

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
								DeviceIgnore:    pointer.To(true),
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
								DeviceInterface: "bond1",
								DeviceBond: &v1alpha1.Bond{
									BondDeviceSelectors: []v1alpha1.NetworkDeviceSelector{{
										NetworkDeviceKernelDriver: kernelDriver,
									}},
									BondMode: "balance-xor",
								},
							},
							{
								DeviceInterface: "eth4",
								DeviceAddresses: []string{"192.168.0.42/24"},
							},
							{
								DeviceInterface: "eth5",
								DeviceAddresses: []string{"192.168.0.43/24"},
							},
							{
								DeviceInterface: "eth8",
								DeviceBridgePort: &v1alpha1.BridgePort{
									BridgePortMaster: "br1",
								},
							},
							{
								DeviceInterface: "br0",
								DeviceBridge: &v1alpha1.Bridge{
									BridgedInterfaces: []string{"eth4", "eth5"},
									BridgeSTP: &v1alpha1.STP{
										STPEnabled: pointer.To(false),
									},
								},
							},
							{
								DeviceInterface: "br1",
								DeviceBridge:    &v1alpha1.Bridge{},
							},
							{
								DeviceInterface: "br0",
								DeviceBridge: &v1alpha1.Bridge{
									BridgeSTP: &v1alpha1.STP{
										STPEnabled: pointer.To(true),
									},
									BridgeVLAN: &v1alpha1.BridgeVLAN{
										BridgeVLANFiltering: pointer.To(true),
									},
								},
							},
							{
								DeviceInterface: "dummy0",
								DeviceDummy:     pointer.To(true),
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
			},
		),
	)

	suite.Create(cfg)

	for _, name := range []string{"eth6", "eth7"} {
		status := network.NewLinkStatus(network.NamespaceName, name)
		status.TypedSpec().Driver = kernelDriver

		suite.Create(status)
	}

	suite.assertLinks(
		[]string{
			"configuration/eth0",
			"configuration/eth0.24",
			"configuration/eth0.48",
			"configuration/eth1",
			"configuration/eth2",
			"configuration/eth3",
			"configuration/eth6",
			"configuration/eth7",
			"configuration/eth8",
			"configuration/bond0",
			"configuration/bond1",
			"configuration/br0",
			"configuration/br1",
			"configuration/dummy0",
			"configuration/wireguard0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			switch r.TypedSpec().Name {
			case "eth0", "eth1":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)

				if r.TypedSpec().Name == "eth0" {
					asrt.EqualValues(0, r.TypedSpec().MTU)
				} else {
					asrt.EqualValues(9001, r.TypedSpec().MTU)
				}
			case "eth0.24", "eth0.48":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
				asrt.Equal("eth0", r.TypedSpec().ParentName)
				asrt.Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)

				if r.TypedSpec().Name == "eth0.24" {
					asrt.EqualValues(24, r.TypedSpec().VLAN.VID)
					asrt.EqualValues(1000, r.TypedSpec().MTU)
				} else {
					asrt.EqualValues(48, r.TypedSpec().VLAN.VID)
					asrt.EqualValues(0, r.TypedSpec().MTU)
				}
			case "eth2", "eth3":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.Equal("bond0", r.TypedSpec().BondSlave.MasterName)
			case "eth6", "eth7":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.Equal("bond1", r.TypedSpec().BondSlave.MasterName)
			case "bond0":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
				asrt.Equal(nethelpers.BondModeXOR, r.TypedSpec().BondMaster.Mode)
				asrt.True(r.TypedSpec().BondMaster.UseCarrier)
			case "bond1":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
				asrt.Equal(nethelpers.BondModeXOR, r.TypedSpec().BondMaster.Mode)
				asrt.True(r.TypedSpec().BondMaster.UseCarrier)
			case "eth4", "eth5":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.Equal("br0", r.TypedSpec().BridgeSlave.MasterName)
			case "eth8":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.Equal("br1", r.TypedSpec().BridgeSlave.MasterName)
			case "br0":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBridge, r.TypedSpec().Kind)
				asrt.True(r.TypedSpec().BridgeMaster.STP.Enabled)
				asrt.True(r.TypedSpec().BridgeMaster.VLAN.FilteringEnabled)
			case "br1":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBridge, r.TypedSpec().Kind)
				asrt.True(r.TypedSpec().BridgeMaster.STP.Enabled)
				asrt.False(r.TypedSpec().BridgeMaster.VLAN.FilteringEnabled)
			case "wireguard0":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkNone, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindWireguard, r.TypedSpec().Kind)
				asrt.Equal(
					network.WireguardSpec{
						PrivateKey: "ABC",
						Peers: []network.WireguardPeer{
							{
								PublicKey: "DEF",
								Endpoint:  "10.0.0.1:3000",
								AllowedIPs: []netip.Prefix{
									netip.MustParsePrefix("10.2.3.0/24"),
									netip.MustParsePrefix("10.2.4.0/24"),
								},
							},
						},
					}, r.TypedSpec().Wireguard,
				)
			}
		},
	)
}

func (suite *LinkConfigSuite) TestMachineConfigurationWithAliases() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

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
								DeviceInterface: "enx0123",
								DeviceVlans: []*v1alpha1.Vlan{
									{
										VlanID:  24,
										VlanMTU: 1000,
									},
								},
							},
							{
								DeviceInterface: "enx0123",
								DeviceMTU:       9001,
							},
							{
								DeviceIgnore:    pointer.To(true),
								DeviceInterface: "enx0456",
							},
							{
								DeviceInterface: "bond0",
								DeviceBond: &v1alpha1.Bond{
									BondInterfaces: []string{"enxa", "enxb"},
									BondMode:       "balance-xor",
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

	for _, link := range []struct {
		name    string
		aliases []string
	}{
		{
			name:    "eth0",
			aliases: []string{"enx0123"},
		},
		{
			name:    "eth1",
			aliases: []string{"enx0456"},
		},
		{
			name:    "eth2",
			aliases: []string{"enxa"},
		},
		{
			name:    "eth3",
			aliases: []string{"enxb"},
		},
	} {
		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().AltNames = link.aliases

		suite.Create(status)
	}

	suite.assertLinks(
		[]string{
			"configuration/eth0",
			"configuration/enx0123.24",
			"configuration/eth2",
			"configuration/eth3",
			"configuration/bond0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			switch r.TypedSpec().Name {
			case "eth0":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.EqualValues(9001, r.TypedSpec().MTU)
			case "eth2", "eth3":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.Equal("bond0", r.TypedSpec().BondSlave.MasterName)
			case "eth0.24":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
				asrt.Equal("eth0", r.TypedSpec().ParentName)
				asrt.Equal(nethelpers.VLANProtocol8021Q, r.TypedSpec().VLAN.Protocol)

				asrt.EqualValues(24, r.TypedSpec().VLAN.VID)
				asrt.EqualValues(1000, r.TypedSpec().MTU)
			case "bond0":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
				asrt.Equal(nethelpers.BondModeXOR, r.TypedSpec().BondMaster.Mode)
				asrt.True(r.TypedSpec().BondMaster.UseCarrier)
			}
		},
	)
}

func (suite *LinkConfigSuite) TestDefaultUp() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.LinkConfigController{
				Cmdline: procfs.NewCmdline("talos.network.interface.ignore=eth2"),
			},
		),
	)

	for _, link := range []string{"eth5", "eth1", "eth2", "eth3", "eth4"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		if link == "eth5" {
			linkStatus.TypedSpec().AltNames = []string{"eth0"}
		}

		suite.Create(linkStatus)
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
			},
		),
	)

	suite.Create(cfg)

	suite.assertLinks(
		[]string{
			"default/eth1",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
			asrt.True(r.TypedSpec().Up)
		},
	)

	suite.assertNoLinks(
		[]string{
			"default/eth0",
			"default/eth2",
			"default/eth3",
			"default/eth4",
		},
	)
}

func TestLinkConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &LinkConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.DeviceConfigController{}))
			},
		},
	})
}
