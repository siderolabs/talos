// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"context"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NetworkConfigSuite ...
type NetworkConfigSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *NetworkConfigSuite) SuiteName() string {
	return "api.NetworkConfigSuite"
}

// SetupTest ...
func (suite *NetworkConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)
}

// TearDownTest ...
func (suite *NetworkConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestStaticHostConfig tests that /etc/hosts updates are working.
func (suite *NetworkConfigSuite) TestStaticHostConfig() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	host1 := network.NewStaticHostConfigV1Alpha1("1.2.3.4")
	host1.Hostnames = []string{"example.com", "example2"}

	host2 := network.NewStaticHostConfigV1Alpha1("2001:db8::1")
	host2.Hostnames = []string{"v6"}

	suite.PatchMachineConfig(nodeCtx, host1, host2)

	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			hosts := suite.ReadFile(nodeCtx, "/etc/hosts")
			scanner := bufio.NewScanner(strings.NewReader(hosts))

			var found1, found2 bool

			for scanner.Scan() {
				line := scanner.Text()

				switch {
				case strings.HasPrefix(line, "1.2.3.4"):
					found1 = true

					asrt.Contains(line, "example.com", "expected to find hostname in IPv4 entry")
					asrt.Contains(line, "example2", "expected to find hostname in IPv4 entry")
				case strings.HasPrefix(line, "2001:db8::1"):
					found2 = true

					asrt.Contains(line, "v6", "expected to find hostname in IPv6 entry")
				}
			}

			asrt.True(found1, "expected to find IPv4 entry in /etc/hosts")
			asrt.True(found2, "expected to find IPv6 entry in /etc/hosts")
		},
		time.Second, time.Millisecond, "waiting for /etc/hosts to be updated",
	)

	suite.RemoveMachineConfigDocuments(nodeCtx, network.StaticHostKind)

	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			hosts := suite.ReadFile(nodeCtx, "/etc/hosts")

			asrt.NotContains(hosts, "1.2.3.4", "expected to not find IPv4 entry in /etc/hosts")
			asrt.NotContains(hosts, "2001:db8::1", "expected to not find IPv6 entry in /etc/hosts")
		},
		time.Second, time.Millisecond, "waiting for /etc/hosts to be updated",
	)
}

// TestDummyLinkConfig tests creation of dummy link interfaces.
func (suite *NetworkConfigSuite) TestDummyLinkConfig() {
	if suite.Cluster == nil {
		suite.T().Skip("skipping if cluster is not qemu/docker")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	dummyName := fmt.Sprintf("dummy%d", rand.IntN(10000))

	dummy := network.NewDummyLinkConfigV1Alpha1(dummyName)
	dummy.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}
	dummy.LinkUp = pointer.To(true)
	dummy.LinkMTU = 9000
	dummy.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1234::1/64"),
			AddressPriority: pointer.To[uint32](100),
		},
	}
	dummy.LinkRoutes = []network.RouteConfig{
		{
			RouteDestination: network.Prefix{Prefix: netip.MustParsePrefix("fd13:1235::/64")},
			RouteGateway:     network.Addr{Addr: netip.MustParseAddr("fd13:1234::ffff")},
		},
	}

	addressID := dummyName + "/fd13:1234::1/64"
	routeID := dummyName + "/inet6/fd13:1234::ffff/fd13:1235::/64/1024"
	addressRouteID := dummyName + "/inet6//fd13:1234::/64/100"

	suite.PatchMachineConfig(nodeCtx, dummy)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, dummyName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.Equal(dummy.HardwareAddressConfig, link.TypedSpec().HardwareAddr)
			asrt.Equal(dummy.LinkMTU, link.TypedSpec().MTU)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(dummyName, addr.TypedSpec().LinkName)
		},
	)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]resource.ID{routeID, addressRouteID},
		func(route *networkres.RouteStatus, asrt *assert.Assertions) {
			asrt.Equal(dummyName, route.TypedSpec().OutLinkName)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.DummyLinkKind, dummyName)

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, dummyName)
	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, addressID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, addressRouteID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, routeID)
}

// TestLinkConfig tests configuring physical links.
func (suite *NetworkConfigSuite) TestLinkConfig() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping if cluster is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	// find the first physical link
	links, err := safe.ReaderListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var linkName string

	for link := range links.All() {
		if link.TypedSpec().Physical() {
			linkName = link.Metadata().ID()

			break
		}
	}

	suite.Require().NotEmpty(linkName, "expected to find at least one physical link")

	cfg := network.NewLinkConfigV1Alpha1(linkName)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1234::2/64"),
			AddressPriority: pointer.To[uint32](2048),
		},
	}

	addressID := linkName + "/fd13:1234::2/64"

	suite.PatchMachineConfig(nodeCtx, cfg)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(linkName, addr.TypedSpec().LinkName)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.LinkKind, linkName)

	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, addressID)
}

// TestLinkAliasConfig tests configuring physical link aliases.
func (suite *NetworkConfigSuite) TestLinkAliasConfig() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping if cluster is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	// find the first physical link without an alias
	links, err := safe.ReaderListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var (
		linkName      string
		permanentAddr string
	)

	for link := range links.All() {
		if link.TypedSpec().Physical() && link.TypedSpec().Alias == "" {
			linkName = link.Metadata().ID()
			permanentAddr = link.TypedSpec().PermanentAddr.String()

			break
		}
	}

	if linkName != "" {
		// we have unaliased physical link to test with, try aliasing it
		const aliasName = "test-alias"

		cfg := network.NewLinkAliasConfigV1Alpha1(aliasName)
		cfg.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression("mac(link.permanent_addr) == '"+permanentAddr+"'", celenv.LinkLocator()))

		suite.PatchMachineConfig(nodeCtx, cfg)

		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(link *networkres.LinkStatus, asrt *assert.Assertions) {
				asrt.Equal(aliasName, link.TypedSpec().Alias)
			},
		)

		suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.LinkAliasKind, aliasName)

		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(link *networkres.LinkStatus, asrt *assert.Assertions) {
				asrt.Empty(link.TypedSpec().Alias)
			},
		)
	} else {
		suite.T().Log("all physical links are already aliased, verifying existing aliases")

		// no unaliased physical links, verify that alias worked properly
		for link := range links.All() {
			if link.TypedSpec().Physical() && link.TypedSpec().Alias != "" {
				rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, link.Metadata().ID(),
					func(linkAlias *networkres.LinkAliasSpec, asrt *assert.Assertions) {
						asrt.Equal(link.TypedSpec().Alias, linkAlias.TypedSpec().Alias)
					},
				)
			}
		}
	}
}

// TestVirtualIPConfig tests configuring virtual IPs.
func (suite *NetworkConfigSuite) TestVirtualIPConfig() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping if cluster is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	// find the first physical link
	links, err := safe.ReaderListAll[*networkres.LinkStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var linkName string

	for link := range links.All() {
		if link.TypedSpec().Physical() {
			linkName = link.Metadata().ID()

			break
		}
	}

	suite.Require().NotEmpty(linkName, "expected to find at least one physical link")

	// using link-local address to avoid kube-apiserver picking it up
	virtualIP := "169.254.100.100"

	cfg := network.NewLayer2VIPConfigV1Alpha1(virtualIP)
	cfg.LinkName = linkName

	addressID := linkName + "/" + virtualIP + "/32"

	suite.PatchMachineConfig(nodeCtx, cfg)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(linkName, addr.TypedSpec().LinkName)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.Layer2VIPKind, virtualIP)

	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, addressID)
}

// TestVLANConfig tests creation of VLAN interfaces.
func (suite *NetworkConfigSuite) TestVLANConfig() {
	if suite.Cluster == nil {
		suite.T().Skip("skipping if cluster is not qemu/docker")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	dummyName := fmt.Sprintf("dummy%d", rand.IntN(10000))

	dummy := network.NewDummyLinkConfigV1Alpha1(dummyName)
	dummy.LinkUp = pointer.To(true)
	dummy.LinkMTU = 9000

	vlanName := dummyName + ".v"

	vlan := network.NewVLANConfigV1Alpha1(vlanName)
	vlan.VLANIDConfig = 100
	vlan.LinkMTU = 2000
	vlan.ParentLinkConfig = dummyName
	vlan.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1234::1/64"),
			AddressPriority: pointer.To[uint32](100),
		},
	}
	vlan.LinkRoutes = []network.RouteConfig{
		{
			RouteDestination: network.Prefix{Prefix: netip.MustParsePrefix("fd13:1235::/64")},
			RouteGateway:     network.Addr{Addr: netip.MustParseAddr("fd13:1234::ffff")},
		},
	}

	addressID := vlanName + "/fd13:1234::1/64"
	routeID := vlanName + "/inet6/fd13:1234::ffff/fd13:1235::/64/1024"
	addressRouteID := vlanName + "/inet6//fd13:1234::/64/100"

	suite.PatchMachineConfig(nodeCtx, dummy, vlan)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, dummyName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.Equal(dummy.LinkMTU, link.TypedSpec().MTU)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, vlanName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("vlan", link.TypedSpec().Kind)
			asrt.Equal(vlan.LinkMTU, link.TypedSpec().MTU)
			asrt.NotZero(link.TypedSpec().LinkIndex)
			asrt.Equal(vlan.VLANIDConfig, link.TypedSpec().VLAN.VID)
			asrt.Equal(nethelpers.VLANProtocol8021Q, link.TypedSpec().VLAN.Protocol)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(vlanName, addr.TypedSpec().LinkName)
		},
	)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]resource.ID{routeID, addressRouteID},
		func(route *networkres.RouteStatus, asrt *assert.Assertions) {
			asrt.Equal(vlanName, route.TypedSpec().OutLinkName)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VLANKind, vlanName)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.DummyLinkKind, dummyName)

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, dummyName)
	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, vlanName)
	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, addressID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, addressRouteID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, routeID)
}

// TestBondConfig tests creation of bond interfaces.
func (suite *NetworkConfigSuite) TestBondConfig() {
	if suite.Cluster == nil {
		suite.T().Skip("skipping if cluster is not qemu/docker")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	dummyNames := xslices.Map([]int{0, 1}, func(int) string {
		return fmt.Sprintf("dummy%d", rand.IntN(10000))
	})

	dummyConfigs := xslices.Map(dummyNames, func(name string) any {
		return network.NewDummyLinkConfigV1Alpha1(name)
	})

	bondName := "agg." + strconv.Itoa(rand.IntN(10000))

	bond := network.NewBondConfigV1Alpha1(bondName)
	bond.BondLinks = dummyNames
	bond.BondMode = pointer.To(nethelpers.BondMode8023AD)
	bond.BondMIIMon = pointer.To(uint32(100))
	bond.BondUpDelay = pointer.To(uint32(200))
	bond.BondDownDelay = pointer.To(uint32(300))
	bond.BondLACPRate = pointer.To(nethelpers.LACPRateSlow)
	bond.BondADActorSysPrio = pointer.To(uint16(65535))
	bond.BondResendIGMP = pointer.To(uint32(1))
	bond.BondPacketsPerSlave = pointer.To(uint32(1))
	bond.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}
	bond.LinkUp = pointer.To(true)
	bond.LinkMTU = 2000
	bond.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1235::1/64"),
			AddressPriority: pointer.To[uint32](100),
		},
	}
	bond.LinkRoutes = []network.RouteConfig{
		{
			RouteDestination: network.Prefix{Prefix: netip.MustParsePrefix("fd13:1236::/64")},
			RouteGateway:     network.Addr{Addr: netip.MustParseAddr("fd13:1235::ffff")},
		},
	}

	addressID := bondName + "/fd13:1235::1/64"
	routeID := bondName + "/inet6/fd13:1235::ffff/fd13:1236::/64/1024"
	addressRouteID := bondName + "/inet6//fd13:1235::/64/100"

	suite.PatchMachineConfig(nodeCtx, append(dummyConfigs, bond)...)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, dummyNames,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.NotZero(link.TypedSpec().MasterIndex)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, bondName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("bond", link.TypedSpec().Kind)
			asrt.Equal(nethelpers.OperStateUp, link.TypedSpec().OperationalState)
			asrt.Equal(bond.LinkMTU, link.TypedSpec().MTU)
			asrt.Equal(nethelpers.BondMode8023AD, link.TypedSpec().BondMaster.Mode)
			asrt.Equal(bond.HardwareAddressConfig, link.TypedSpec().HardwareAddr)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(bondName, addr.TypedSpec().LinkName)
		},
	)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]resource.ID{routeID, addressRouteID},
		func(route *networkres.RouteStatus, asrt *assert.Assertions) {
			asrt.Equal(bondName, route.TypedSpec().OutLinkName)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.BondKind, bondName)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.DummyLinkKind, dummyNames...)

	for _, dummyName := range dummyNames {
		rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, dummyName)
	}

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, bondName)
	rtestutils.AssertNoResource[*networkres.AddressStatus](nodeCtx, suite.T(), suite.Client.COSI, addressID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, addressRouteID)
	rtestutils.AssertNoResource[*networkres.RouteStatus](nodeCtx, suite.T(), suite.Client.COSI, routeID)
}

// TestBridgeConfig tests creation of bridge interfaces.
func (suite *NetworkConfigSuite) TestBridgeConfig() {
	if suite.Cluster == nil {
		suite.T().Skip("skipping if cluster is not qemu/docker")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	dummyNames := xslices.Map([]int{0, 1}, func(int) string {
		return fmt.Sprintf("dummy%d", rand.IntN(10000))
	})

	dummyConfigs := xslices.Map(dummyNames, func(name string) any {
		return network.NewDummyLinkConfigV1Alpha1(name)
	})

	bridgeName := "bridge." + strconv.Itoa(rand.IntN(10000))

	bridge := network.NewBridgeConfigV1Alpha1(bridgeName)
	bridge.BridgeLinks = dummyNames
	bridge.BridgeSTP.BridgeSTPEnabled = pointer.To(true)
	bridge.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}
	bridge.LinkUp = pointer.To(true)

	suite.PatchMachineConfig(nodeCtx, append(dummyConfigs, bridge)...)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, dummyNames,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.NotZero(link.TypedSpec().MasterIndex)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, bridgeName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("bridge", link.TypedSpec().Kind)
			asrt.Equal(pointer.SafeDeref(bridge.BridgeSTP.BridgeSTPEnabled), link.TypedSpec().BridgeMaster.STP.Enabled)
			asrt.Equal(bridge.HardwareAddressConfig, link.TypedSpec().HardwareAddr)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.BridgeKind, bridgeName)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.DummyLinkKind, dummyNames...)

	for _, dummyName := range dummyNames {
		rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, dummyName)
	}

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, bridgeName)
}

// TestWireguardConfig tests creation of Wireguard interfaces.
func (suite *NetworkConfigSuite) TestWireguardConfig() {
	if suite.Cluster == nil {
		suite.T().Skip("skipping if cluster is not qemu/docker")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	wgName := "wg." + strconv.Itoa(rand.IntN(10000))

	privateKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	peerKey, err := wgtypes.GeneratePrivateKey()
	suite.Require().NoError(err)

	wg := network.NewWireguardConfigV1Alpha1(wgName)
	wg.WireguardPrivateKey = privateKey.String()
	wg.WireguardListenPort = 3042
	wg.WireguardPeers = []network.WireguardPeer{
		{
			WireguardPublicKey: peerKey.PublicKey().String(),
			WireguardAllowedIPs: []network.Prefix{
				{
					Prefix: netip.MustParsePrefix("192.168.2.0/24"),
				},
			},
		},
	}
	wg.LinkUp = pointer.To(true)

	suite.PatchMachineConfig(nodeCtx, wg)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, wgName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("wireguard", link.TypedSpec().Kind)
			asrt.Equal(wg.WireguardListenPort, link.TypedSpec().Wireguard.ListenPort)
			asrt.Len(link.TypedSpec().Wireguard.Peers, 1)
			asrt.Equal(peerKey.PublicKey().String(), link.TypedSpec().Wireguard.Peers[0].PublicKey)
			asrt.Equal(privateKey.PublicKey().String(), link.TypedSpec().Wireguard.PublicKey)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.WireguardKind, wgName)

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, wgName)
}

func init() {
	allSuites = append(allSuites, new(NetworkConfigSuite))
}
