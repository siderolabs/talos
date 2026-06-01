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
	dummy.LinkUp = new(true)
	dummy.LinkMTU = 9000
	dummy.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1234::1/64"),
			AddressPriority: new(uint32(100)),
		},
	}
	dummy.LinkRoutes = []network.RouteConfig{
		{
			RouteDestination: network.Prefix{Prefix: netip.MustParsePrefix("fd13:1235::/64")},
			RouteGateway:     network.Addr{Addr: netip.MustParseAddr("fd13:1234::ffff")},
			RouteTable:       nethelpers.Table101,
		},
	}

	addressID := dummyName + "/fd13:1234::1/64"
	routeID := "101/" + dummyName + "/inet6/fd13:1234::ffff/fd13:1235::/64/1024"
	addressRouteID := dummyName + "/inet6//fd13:1234::/64/100"

	suite.PatchMachineConfig(nodeCtx, dummy)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, dummyName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.Equal(dummy.HardwareAddressConfig, link.TypedSpec().HardwareAddr)
			asrt.Equal(dummy.LinkMTU, link.TypedSpec().MTU)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(dummyName, addr.TypedSpec().LinkName)
		},
	)

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI,
		[]resource.ID{routeID, addressRouteID},
		func(route *networkres.RouteStatus, asrt *assert.Assertions) {
			asrt.Equal(dummyName, route.TypedSpec().OutLinkName)

			if route.Metadata().ID() == routeID {
				asrt.Equal(nethelpers.Table101, route.TypedSpec().Table)
			} else {
				asrt.Equal(nethelpers.TableMain, route.TypedSpec().Table)
			}
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
			AddressPriority: new(uint32(2048)),
		},
	}

	addressID := linkName + "/fd13:1234::2/64"

	suite.PatchMachineConfig(nodeCtx, cfg)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, addressID,
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

		rtestutils.AssertResource(
			nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(link *networkres.LinkStatus, asrt *assert.Assertions) {
				asrt.Equal(aliasName, link.TypedSpec().Alias)
			},
		)

		suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.LinkAliasKind, aliasName)

		rtestutils.AssertResource(
			nodeCtx, suite.T(), suite.Client.COSI, linkName,
			func(link *networkres.LinkStatus, asrt *assert.Assertions) {
				asrt.Empty(link.TypedSpec().Alias)
			},
		)
	} else {
		suite.T().Log("all physical links are already aliased, verifying existing aliases")

		// no unaliased physical links, verify that alias worked properly
		for link := range links.All() {
			if link.TypedSpec().Physical() && link.TypedSpec().Alias != "" {
				rtestutils.AssertResource(
					nodeCtx, suite.T(), suite.Client.COSI, link.Metadata().ID(),
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

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, addressID,
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
	dummy.LinkUp = new(true)
	dummy.LinkMTU = 9000

	vlanName := dummyName + ".v"

	vlan := network.NewVLANConfigV1Alpha1(vlanName)
	vlan.VLANIDConfig = 100
	vlan.LinkMTU = 2000
	vlan.ParentLinkConfig = dummyName
	vlan.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1234::1/64"),
			AddressPriority: new(uint32(100)),
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

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, dummyName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.Equal(dummy.LinkMTU, link.TypedSpec().MTU)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, vlanName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("vlan", link.TypedSpec().Kind)
			asrt.Equal(vlan.LinkMTU, link.TypedSpec().MTU)
			asrt.NotZero(link.TypedSpec().LinkIndex)
			asrt.Equal(vlan.VLANIDConfig, link.TypedSpec().VLAN.VID)
			asrt.Equal(nethelpers.VLANProtocol8021Q, link.TypedSpec().VLAN.Protocol)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(vlanName, addr.TypedSpec().LinkName)
		},
	)

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI,
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
	bond.BondMode = new(nethelpers.BondMode8023AD)
	bond.BondMIIMon = new(uint32(100))
	bond.BondUpDelay = new(uint32(200))
	bond.BondDownDelay = new(uint32(300))
	bond.BondLACPRate = new(nethelpers.LACPRateSlow)
	bond.BondADActorSysPrio = new(uint16(65535))
	bond.BondResendIGMP = new(uint32(1))
	bond.BondPacketsPerSlave = new(uint32(1))
	bond.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}
	bond.LinkUp = new(true)
	bond.LinkMTU = 2000
	bond.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress:  netip.MustParsePrefix("fd13:1235::1/64"),
			AddressPriority: new(uint32(100)),
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

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI, dummyNames,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.NotZero(link.TypedSpec().MasterIndex)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, bondName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("bond", link.TypedSpec().Kind)
			asrt.Equal(nethelpers.OperStateUp, link.TypedSpec().OperationalState)
			asrt.Equal(bond.LinkMTU, link.TypedSpec().MTU)
			asrt.Equal(nethelpers.BondMode8023AD, link.TypedSpec().BondMaster.Mode)
			asrt.Equal(bond.HardwareAddressConfig, link.TypedSpec().HardwareAddr)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, addressID,
		func(addr *networkres.AddressStatus, asrt *assert.Assertions) {
			asrt.Equal(bondName, addr.TypedSpec().LinkName)
		},
	)

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI,
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
	bridge.BridgeSTP.BridgeSTPEnabled = new(true)
	bridge.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}
	bridge.LinkUp = new(true)

	suite.PatchMachineConfig(nodeCtx, append(dummyConfigs, bridge)...)

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI, dummyNames,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.NotZero(link.TypedSpec().MasterIndex)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, bridgeName,
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

// TestVRFConfig tests creation of vrf interfaces.
func (suite *NetworkConfigSuite) TestVRFConfig() {
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

	vrfName := "vrf." + strconv.Itoa(rand.IntN(10000))

	vrf := network.NewVRFConfigV1Alpha1(vrfName)
	vrf.VRFLinks = dummyNames
	vrf.VRFTable = nethelpers.RoutingTable(123)
	vrf.HardwareAddressConfig = nethelpers.HardwareAddr{0x02, 0x00, 0x00, 0x00, byte(rand.IntN(256)), byte(rand.IntN(256))}
	vrf.LinkUp = new(true)

	suite.PatchMachineConfig(nodeCtx, append(dummyConfigs, vrf)...)

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI, dummyNames,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("dummy", link.TypedSpec().Kind)
			asrt.NotZero(link.TypedSpec().MasterIndex)
		},
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, vrfName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("vrf", link.TypedSpec().Kind)
			asrt.Equal(vrf.VRFTable, link.TypedSpec().VRFMaster.Table)
			asrt.Equal(vrf.HardwareAddressConfig, link.TypedSpec().HardwareAddr)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.VRFKind, vrfName)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.DummyLinkKind, dummyNames...)

	for _, dummyName := range dummyNames {
		rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, dummyName)
	}

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, vrfName)
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
	wg.LinkUp = new(true)

	suite.PatchMachineConfig(nodeCtx, wg)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, wgName,
		func(link *networkres.LinkStatus, asrt *assert.Assertions) {
			asrt.Equal("wireguard", link.TypedSpec().Kind)
			asrt.Equal(wg.WireguardListenPort, link.TypedSpec().Wireguard.ListenPort)

			if asrt.Len(link.TypedSpec().Wireguard.Peers, 1) {
				asrt.Equal(peerKey.PublicKey().String(), link.TypedSpec().Wireguard.Peers[0].PublicKey)
			}

			asrt.Equal(privateKey.PublicKey().String(), link.TypedSpec().Wireguard.PublicKey)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.WireguardKind, wgName)

	rtestutils.AssertNoResource[*networkres.LinkStatus](nodeCtx, suite.T(), suite.Client.COSI, wgName)
}

// TestBlackholeRouteConfig tests creation of blackhole routes.
func (suite *NetworkConfigSuite) TestBlackholeRouteConfig() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing on node %q", node)

	const dest = "fd13:1236::/64"

	cfg := network.NewBlackholeRouteConfigV1Alpha1(dest)
	suite.PatchMachineConfig(nodeCtx, cfg)

	const routeBlackholeID = "lo/inet6//" + dest + "/1024"

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI,
		[]resource.ID{routeBlackholeID},
		func(route *networkres.RouteStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.TypeBlackhole, route.TypedSpec().Type)
		},
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.BlackholeRouteKind, dest)
}

// routingRuleTestCtx creates a context with a longer timeout for routing rule tests,
// because the RoutingRuleStatusController uses 30s polling (no rtnetlink multicast group for rule changes).
func (suite *NetworkConfigSuite) routingRuleTestCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Minute)
}

// requiredSystemRoutingTables lists the system tables a working node must
// reach via some routing rule, per family. Cilium (and similar CNIs) may move
// the local lookup rule from the kernel-default priority 0 to a higher one, so
// the test only asserts that *some* rule below `main` (32766) lookups each
// required table - never the exact rule ID.
var requiredSystemRoutingTables = map[nethelpers.Family][]nethelpers.RoutingTable{
	nethelpers.FamilyInet4: {nethelpers.TableLocal, nethelpers.TableMain, nethelpers.TableDefault},
	nethelpers.FamilyInet6: {nethelpers.TableLocal, nethelpers.TableMain},
}

// assertKernelDefaultRoutingRulesPresent asserts that policy routing remains
// functional: for every required system table (local/main/default) some
// routing rule looks it up. This permits CNIs like Cilium to relocate the
// local lookup from priority 0 to 100 while still catching cases where Talos
// accidentally deletes a kernel-installed rule. Every routing-rule
// integration test calls this both before applying its config and after
// teardown.
func (suite *NetworkConfigSuite) assertKernelDefaultRoutingRulesPresent(nodeCtx context.Context) {
	suite.T().Helper()

	suite.EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			rules, err := safe.StateListAll[*networkres.RoutingRuleStatus](nodeCtx, suite.Client.COSI)
			if !asrt.NoError(err, "listing routing rule statuses") {
				return
			}

			seen := map[nethelpers.Family]map[nethelpers.RoutingTable]bool{}

			for rule := range rules.All() {
				spec := rule.TypedSpec()

				if seen[spec.Family] == nil {
					seen[spec.Family] = map[nethelpers.RoutingTable]bool{}
				}

				seen[spec.Family][spec.Table] = true
			}

			for family, tables := range requiredSystemRoutingTables {
				for _, table := range tables {
					asrt.Truef(seen[family][table],
						"no rule looks up table %s for family %s", table, family)
				}
			}
		},
		30*time.Second, time.Second,
		"waiting for system routing rules (local/main/default lookups) to be present",
	)
}

// TestRoutingRuleBasic tests creation of a routing rule with a numeric table ID.
//
//nolint:dupl
func (suite *NetworkConfigSuite) TestRoutingRuleBasic() {
	ctx, cancel := suite.routingRuleTestCtx()
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(ctx, node)

	suite.T().Logf("testing routing rule (basic) on node %q", node)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	const rulePriority uint32 = 1000

	cfg := network.NewRoutingRuleConfigV1Alpha1(rulePriority)
	cfg.RuleSrc = network.Prefix{Prefix: netip.MustParsePrefix("10.99.0.0/16")}
	cfg.RuleTable = nethelpers.RoutingTable(100)

	suite.PatchMachineConfig(nodeCtx, cfg)

	const ruleStatusID = "inet4/01000"

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID,
		func(rule *networkres.RoutingRuleStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.FamilyInet4, rule.TypedSpec().Family)
			asrt.Equal(netip.MustParsePrefix("10.99.0.0/16"), rule.TypedSpec().Src)
			asrt.Equal(nethelpers.RoutingTable(100), rule.TypedSpec().Table)
			asrt.Equal(uint32(1000), rule.TypedSpec().Priority)
		},
	)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.RoutingRuleKind, strconv.FormatUint(uint64(rulePriority), 10))

	rtestutils.AssertNoResource[*networkres.RoutingRuleStatus](nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)
}

// TestRoutingRuleIPv6 tests creation of an IPv6 routing rule.
//
//nolint:dupl
func (suite *NetworkConfigSuite) TestRoutingRuleIPv6() {
	ctx, cancel := suite.routingRuleTestCtx()
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(ctx, node)

	suite.T().Logf("testing routing rule (IPv6) on node %q", node)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	const rulePriority uint32 = 3000

	cfg := network.NewRoutingRuleConfigV1Alpha1(rulePriority)
	cfg.RuleSrc = network.Prefix{Prefix: netip.MustParsePrefix("fd99:1234::/48")}
	cfg.RuleTable = nethelpers.RoutingTable(100)

	suite.PatchMachineConfig(nodeCtx, cfg)

	const ruleStatusID = "inet6/03000"

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID,
		func(rule *networkres.RoutingRuleStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.FamilyInet6, rule.TypedSpec().Family)
			asrt.Equal(netip.MustParsePrefix("fd99:1234::/48"), rule.TypedSpec().Src)
			asrt.Equal(nethelpers.RoutingTable(100), rule.TypedSpec().Table)
			asrt.Equal(uint32(3000), rule.TypedSpec().Priority)
		},
	)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.RoutingRuleKind, strconv.FormatUint(uint64(rulePriority), 10))

	rtestutils.AssertNoResource[*networkres.RoutingRuleStatus](nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)
}

// TestRoutingRuleSrcAndDst tests creation of a routing rule with both source and destination prefixes.
func (suite *NetworkConfigSuite) TestRoutingRuleSrcAndDst() {
	ctx, cancel := suite.routingRuleTestCtx()
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(ctx, node)

	suite.T().Logf("testing routing rule (src+dst) on node %q", node)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	const rulePriority uint32 = 4000

	cfg := network.NewRoutingRuleConfigV1Alpha1(rulePriority)
	cfg.RuleSrc = network.Prefix{Prefix: netip.MustParsePrefix("10.96.0.0/16")}
	cfg.RuleDst = network.Prefix{Prefix: netip.MustParsePrefix("192.168.99.0/24")}
	cfg.RuleTable = nethelpers.RoutingTable(100)

	suite.PatchMachineConfig(nodeCtx, cfg)

	const ruleStatusID = "inet4/04000"

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID,
		func(rule *networkres.RoutingRuleStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.FamilyInet4, rule.TypedSpec().Family)
			asrt.Equal(netip.MustParsePrefix("10.96.0.0/16"), rule.TypedSpec().Src)
			asrt.Equal(netip.MustParsePrefix("192.168.99.0/24"), rule.TypedSpec().Dst)
			asrt.Equal(nethelpers.RoutingTable(100), rule.TypedSpec().Table)
			asrt.Equal(uint32(4000), rule.TypedSpec().Priority)
		},
	)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.RoutingRuleKind, strconv.FormatUint(uint64(rulePriority), 10))

	rtestutils.AssertNoResource[*networkres.RoutingRuleStatus](nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)
}

// TestRoutingRuleBlackholeAction tests creation of a routing rule with blackhole action.
func (suite *NetworkConfigSuite) TestRoutingRuleBlackholeAction() {
	ctx, cancel := suite.routingRuleTestCtx()
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(ctx, node)

	suite.T().Logf("testing routing rule (blackhole action) on node %q", node)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	const rulePriority uint32 = 5000

	cfg := network.NewRoutingRuleConfigV1Alpha1(rulePriority)
	cfg.RuleSrc = network.Prefix{Prefix: netip.MustParsePrefix("10.95.0.0/16")}
	cfg.RuleTable = nethelpers.RoutingTable(100)
	cfg.RuleAction = nethelpers.RoutingRuleActionBlackhole

	suite.PatchMachineConfig(nodeCtx, cfg)

	const ruleStatusID = "inet4/05000"

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID,
		func(rule *networkres.RoutingRuleStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.FamilyInet4, rule.TypedSpec().Family)
			asrt.Equal(nethelpers.RoutingRuleActionBlackhole, rule.TypedSpec().Action)
			asrt.Equal(uint32(5000), rule.TypedSpec().Priority)
		},
	)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.RoutingRuleKind, strconv.FormatUint(uint64(rulePriority), 10))

	rtestutils.AssertNoResource[*networkres.RoutingRuleStatus](nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)
}

// TestRoutingRuleFwMark tests creation of a routing rule with firewall mark matching.
func (suite *NetworkConfigSuite) TestRoutingRuleFwMark() {
	ctx, cancel := suite.routingRuleTestCtx()
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(ctx, node)

	suite.T().Logf("testing routing rule (fwmark) on node %q", node)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	const rulePriority uint32 = 6000

	cfg := network.NewRoutingRuleConfigV1Alpha1(rulePriority)
	cfg.RuleSrc = network.Prefix{Prefix: netip.MustParsePrefix("10.94.0.0/16")}
	cfg.RuleTable = nethelpers.RoutingTable(100)
	cfg.RuleFwMark = 0x100
	cfg.RuleFwMask = 0xff00

	suite.PatchMachineConfig(nodeCtx, cfg)

	const ruleStatusID = "inet4/06000"

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID,
		func(rule *networkres.RoutingRuleStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.FamilyInet4, rule.TypedSpec().Family)
			asrt.Equal(nethelpers.RoutingTable(100), rule.TypedSpec().Table)
			asrt.Equal(uint32(6000), rule.TypedSpec().Priority)
			asrt.Equal(uint32(0x100), rule.TypedSpec().FwMark)
			asrt.Equal(uint32(0xff00), rule.TypedSpec().FwMask)
		},
	)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, network.RoutingRuleKind, strconv.FormatUint(uint64(rulePriority), 10))

	rtestutils.AssertNoResource[*networkres.RoutingRuleStatus](nodeCtx, suite.T(), suite.Client.COSI, ruleStatusID)

	suite.assertKernelDefaultRoutingRulesPresent(nodeCtx)
}

func init() {
	allSuites = append(allSuites, new(NetworkConfigSuite))
}
