// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:goconst,dupl
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
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
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
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
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
								DeviceIgnore:    new(true),
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
										STPEnabled: new(false),
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
										STPEnabled: new(true),
									},
									BridgeVLAN: &v1alpha1.BridgeVLAN{
										BridgeVLANFiltering: new(true),
									},
								},
							},
							{
								DeviceInterface: "dummy0",
								DeviceDummy:     new(true),
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

				asrt.Nil(r.TypedSpec().Multicast)
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
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
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
								DeviceIgnore:    new(true),
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
				asrt.Equal(nethelpers.ADLACPActiveOn, r.TypedSpec().BondMaster.ADLACPActive)
			}
		},
	)
}

func (suite *LinkConfigSuite) TestMachineConfigurationNewStyle() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

	lc1 := networkcfg.NewLinkConfigV1Alpha1("enp0s2")
	lc1.LinkMTU = 9001

	dc1 := networkcfg.NewDummyLinkConfigV1Alpha1("dummy1")
	dc1.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x42, 0xac, 0x11, 0x00, 0x02}
	dc1.LinkUp = new(true)

	vl1 := networkcfg.NewVLANConfigV1Alpha1("dummy1.100")
	vl1.VLANIDConfig = 100
	vl1.ParentLinkConfig = "dummy1"
	vl1.VLANModeConfig = new(nethelpers.VLANProtocol8021AD)
	vl1.LinkMTU = 200
	vl1.LinkUp = new(true)

	dc2 := networkcfg.NewDummyLinkConfigV1Alpha1("dummy2")
	dc3 := networkcfg.NewDummyLinkConfigV1Alpha1("dummy3")

	bc1 := networkcfg.NewBondConfigV1Alpha1("bond357")
	bc1.BondMode = new(nethelpers.BondModeActiveBackup)
	bc1.BondLinks = []string{"dummy2", "dummy3"}
	bc1.BondUpDelay = new(uint32(200))

	br1 := networkcfg.NewBridgeConfigV1Alpha1("br0")
	br1.BridgeLinks = []string{"enp0s2", "eth1"}
	br1.BridgeSTP.BridgeSTPEnabled = new(true)
	br1.BridgeVLAN.BridgeVLANFiltering = new(true)

	ctr, err := container.New(dc1, lc1, vl1, dc2, dc3, bc1, br1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	for _, link := range []struct {
		name    string
		aliases []string
	}{
		{
			name:    "eth0",
			aliases: []string{"enp0s2"},
		},
	} {
		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().AltNames = link.aliases

		suite.Create(status)
	}

	suite.assertLinks(
		[]string{
			"configuration/eth0",
			"configuration/dummy1",
			"configuration/dummy2",
			"configuration/dummy3",
			"configuration/dummy1.100",
			"configuration/bond357",
			"configuration/br0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			switch r.TypedSpec().Name {
			case "eth0":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.EqualValues(9001, r.TypedSpec().MTU)
				asrt.Equal("br0", r.TypedSpec().BridgeSlave.MasterName)
			case "eth1":
				asrt.True(r.TypedSpec().Up)
				asrt.Equal("br0", r.TypedSpec().BridgeSlave.MasterName)
			case "dummy1", "dummy2", "dummy3":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal("dummy", r.TypedSpec().Kind)

				if r.TypedSpec().Name == "dummy2" || r.TypedSpec().Name == "dummy3" {
					asrt.Equal("bond357", r.TypedSpec().BondSlave.MasterName)
				}
			case "dummy1.100":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindVLAN, r.TypedSpec().Kind)
				asrt.Equal("dummy1", r.TypedSpec().ParentName)
				asrt.Equal(nethelpers.VLANProtocol8021AD, r.TypedSpec().VLAN.Protocol)
				asrt.EqualValues(100, r.TypedSpec().VLAN.VID)
				asrt.EqualValues(200, r.TypedSpec().MTU)
			case "bond357":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBond, r.TypedSpec().Kind)
				asrt.Equal(nethelpers.BondModeActiveBackup, r.TypedSpec().BondMaster.Mode)
				asrt.EqualValues(200, r.TypedSpec().BondMaster.UpDelay)
				asrt.Equal(nethelpers.ADLACPActiveOn, r.TypedSpec().BondMaster.ADLACPActive)
			case "br0":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindBridge, r.TypedSpec().Kind)
				asrt.True(r.TypedSpec().BridgeMaster.STP.Enabled)
				asrt.True(r.TypedSpec().BridgeMaster.VLAN.FilteringEnabled)
			}
		},
	)
}

func (suite *LinkConfigSuite) TestMachineConfigurationNewStyleVRF() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

	lc1 := networkcfg.NewLinkConfigV1Alpha1("enp0s2")
	lc1.LinkMTU = 9001

	dc1 := networkcfg.NewDummyLinkConfigV1Alpha1("dummy1")
	dc1.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x42, 0xac, 0x11, 0x00, 0x02}
	dc1.LinkUp = new(true)

	vrf := networkcfg.NewVRFConfigV1Alpha1("vrf-blue")
	vrf.VRFLinks = []string{"enp0s2", "dummy1"}
	vrf.VRFTable = nethelpers.Table123

	ctr, err := container.New(lc1, dc1, vrf)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	for _, link := range []struct {
		name    string
		aliases []string
	}{
		{
			name:    "eth0",
			aliases: []string{"enp0s2"},
		},
	} {
		status := network.NewLinkStatus(network.NamespaceName, link.name)
		status.TypedSpec().AltNames = link.aliases

		suite.Create(status)
	}

	suite.assertLinks(
		[]string{
			"configuration/eth0",
			"configuration/dummy1",
			"configuration/vrf-blue",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			switch r.TypedSpec().Name {
			case "eth0":
				asrt.True(r.TypedSpec().Up)
				asrt.False(r.TypedSpec().Logical)
				asrt.EqualValues(9001, r.TypedSpec().MTU)
				asrt.Equal("vrf-blue", r.TypedSpec().VRFSlave.MasterName)
			case "dummy1":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal("dummy", r.TypedSpec().Kind)
				asrt.Equal("vrf-blue", r.TypedSpec().VRFSlave.MasterName)
			case "vrf-blue":
				asrt.True(r.TypedSpec().Up)
				asrt.True(r.TypedSpec().Logical)
				asrt.Equal(nethelpers.LinkEther, r.TypedSpec().Type)
				asrt.Equal(network.LinkKindVRF, r.TypedSpec().Kind)
				asrt.Equal(nethelpers.Table123, r.TypedSpec().VRFMaster.Table)
			}
		},
	)
}

func (suite *LinkConfigSuite) TestMachineConfigurationNewStyleNotFIPS() {
	if fipsmode.Strict() {
		suite.T().Skip("skipping test in strict FIPS mode")
	}

	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

	privKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	pskKey, err := wgtypes.GenerateKey()
	suite.Require().NoError(err)

	peerKey, err := wgtypes.GenerateKey()
	suite.Require().NoError(err)

	wc1 := networkcfg.NewWireguardConfigV1Alpha1("wg0")
	wc1.LinkUp = new(true)
	wc1.WireguardPrivateKey = privKey.String()
	wc1.WireguardListenPort = 12345
	wc1.WireguardPeers = []networkcfg.WireguardPeer{
		{
			WireguardPublicKey:    peerKey.PublicKey().String(),
			WireguardPresharedKey: pskKey.String(),
			WireguardAllowedIPs:   []networkcfg.Prefix{{Prefix: netip.MustParsePrefix("10.0.0.0/24")}},
		},
	}

	ctr, err := container.New(wc1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.assertLinks(
		[]string{
			"configuration/wg0",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			asrt.True(r.TypedSpec().Up)
			asrt.True(r.TypedSpec().Logical)
			asrt.Equal(nethelpers.LinkNone, r.TypedSpec().Type)
			asrt.Equal(network.LinkKindWireguard, r.TypedSpec().Kind)
			asrt.Equal(
				network.WireguardSpec{
					PrivateKey:   privKey.String(),
					ListenPort:   12345,
					FirewallMark: 0,
					Peers: []network.WireguardPeer{
						{
							PublicKey:    peerKey.PublicKey().String(),
							PresharedKey: pskKey.String(),
							AllowedIPs: []netip.Prefix{
								netip.MustParsePrefix("10.0.0.0/24"),
							},
						},
					},
				},
				r.TypedSpec().Wireguard,
			)
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
			linkStatus.TypedSpec().AltNames = []string{"enp0s2"}
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
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
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

func (suite *LinkConfigSuite) TestNoDefaultUp() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

	for _, link := range []string{"eth5", "eth1", "eth2", "eth3", "eth4"} {
		linkStatus := network.NewLinkStatus(network.NamespaceName, link)
		linkStatus.TypedSpec().Type = nethelpers.LinkEther
		linkStatus.TypedSpec().LinkState = true

		if link == "eth5" {
			linkStatus.TypedSpec().AltNames = []string{"eth0"}
		}

		suite.Create(linkStatus)
	}

	// default link up
	suite.assertLinks(
		[]string{
			"default/eth1",
			"default/eth2",
			"default/eth3",
			"default/eth4",
			"default/eth5",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigDefault, r.TypedSpec().ConfigLayer)
			asrt.True(r.TypedSpec().Up)
		},
	)

	// create config
	lc1 := networkcfg.NewLinkConfigV1Alpha1("enp0s2")
	lc1.LinkMTU = 9001

	ctr, err := container.New(lc1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	// no default links up since we have config now
	suite.assertNoLinks(
		[]string{
			"default/eth0",
			"default/eth1",
			"default/eth2",
			"default/eth3",
			"default/eth4",
			"default/eth5",
		},
	)
}

func (suite *LinkConfigSuite) TestMulticast() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkConfigController{}))

	lc1 := networkcfg.NewLinkConfigV1Alpha1("enp1s1")
	lc1.LinkMulticast = new(false)
	lc2 := networkcfg.NewLinkConfigV1Alpha1("enp1s2")
	lc2.LinkMulticast = new(true)

	ctr, err := container.New(lc1, lc2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	suite.assertLinks(
		[]string{
			"configuration/enp1s1",
			"configuration/enp1s2",
		}, func(r *network.LinkSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)

			switch r.TypedSpec().Name {
			case "enp1s1":
				asrt.False(*r.TypedSpec().Multicast)
			case "enp1s2":
				asrt.True(*r.TypedSpec().Multicast)
			}
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
