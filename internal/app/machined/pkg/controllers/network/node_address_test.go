// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/addressutil"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type NodeAddressSuite struct {
	ctest.DefaultSuite
}

func (suite *NodeAddressSuite) TestDefaults() {
	// create fake device ready status
	deviceStatus := runtimeres.NewDevicesStatus(runtimeres.NamespaceName, runtimeres.DevicesID)
	deviceStatus.TypedSpec().Ready = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), deviceStatus))

	sortAlgorithm := network.NewNodeAddressSortAlgorithm(network.NamespaceName, network.NodeAddressSortAlgorithmID)
	sortAlgorithm.TypedSpec().Algorithm = nethelpers.AddressSortAlgorithmV1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), sortAlgorithm))

	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.AddressStatusController{}))
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.LinkStatusController{}))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressRoutedID,
			network.NodeAddressAccumulativeID,
		},
		func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			suite.T().Logf("id %q val %s", r.Metadata().ID(), addrs)

			asrt.True(
				slices.IsSortedFunc(
					addrs,
					addressutil.ComparePrefixesLegacy,
				), "addresses %s", addrs,
			)

			if r.Metadata().ID() == network.NodeAddressDefaultID {
				asrt.Len(addrs, 1)
			} else {
				asrt.NotEmpty(addrs)
			}
		},
	)
}

func (suite *NodeAddressSuite) newAddress(addr netip.Prefix, link *network.LinkStatus) {
	var addressStatusController netctrl.AddressStatusController

	addressStatus := network.NewAddressStatus(network.NamespaceName, network.AddressID(link.Metadata().ID(), addr))
	addressStatus.TypedSpec().Address = addr
	addressStatus.TypedSpec().LinkName = link.Metadata().ID()
	addressStatus.TypedSpec().LinkIndex = link.TypedSpec().Index
	suite.Require().NoError(
		suite.State().Create(
			suite.Ctx(),
			addressStatus,
			state.WithCreateOwner(addressStatusController.Name()),
		),
	)
}

func (suite *NodeAddressSuite) newExternalAddress(addr netip.Prefix) {
	var platformConfigController netctrl.PlatformConfigController

	addressStatus := network.NewAddressStatus(network.NamespaceName, network.AddressID("external", addr))
	addressStatus.TypedSpec().Address = addr
	addressStatus.TypedSpec().LinkName = "external"
	suite.Require().NoError(
		suite.State().Create(
			suite.Ctx(),
			addressStatus,
			state.WithCreateOwner(platformConfigController.Name()),
		),
	)
}

//nolint:gocyclo
func (suite *NodeAddressSuite) TestFilters() {
	linkUp := network.NewLinkStatus(network.NamespaceName, "eth0")
	linkUp.TypedSpec().Type = nethelpers.LinkEther
	linkUp.TypedSpec().LinkState = true
	linkUp.TypedSpec().Index = 1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), linkUp))

	linkDown := network.NewLinkStatus(network.NamespaceName, "eth1")
	linkDown.TypedSpec().Type = nethelpers.LinkEther
	linkDown.TypedSpec().LinkState = false
	linkDown.TypedSpec().Index = 2
	suite.Require().NoError(suite.State().Create(suite.Ctx(), linkDown))

	sortAlgorithm := network.NewNodeAddressSortAlgorithm(network.NamespaceName, network.NodeAddressSortAlgorithmID)
	sortAlgorithm.TypedSpec().Algorithm = nethelpers.AddressSortAlgorithmV1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), sortAlgorithm))

	for _, addr := range []string{
		"10.0.0.1/8",
		"25.3.7.9/32",
		"2001:470:6d:30e:4a62:b3ba:180b:b5b8/64",
		"127.0.0.1/8",
		"fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
	} {
		suite.newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	for _, addr := range []string{"10.0.0.2/8", "192.168.3.7/24"} {
		suite.newAddress(netip.MustParsePrefix(addr), linkDown)
	}

	for _, addr := range []string{"1.2.3.4/32", "25.3.7.9/32"} { // duplicate with link address: 25.3.7.9
		suite.newExternalAddress(netip.MustParsePrefix(addr))
	}

	filter1 := network.NewNodeAddressFilter(network.NamespaceName, "no-k8s")
	filter1.TypedSpec().ExcludeSubnets = []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), filter1))

	filter2 := network.NewNodeAddressFilter(network.NamespaceName, "only-k8s")
	filter2.TypedSpec().IncludeSubnets = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), filter2))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressRoutedID,
			network.NodeAddressAccumulativeID,
			network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()),
		},
		func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressDefaultID:
				asrt.Equal("10.0.0.1/8", stringifyIPs(addrs))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					"1.2.3.4/32 10.0.0.1/8 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
					stringifyIPs(addrs),
				)
			case network.NodeAddressRoutedID:
				asrt.Equal(
					"10.0.0.1/8 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64",
					stringifyIPs(addrs),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					"1.2.3.4/32 10.0.0.1/8 10.0.0.2/8 25.3.7.9/32 192.168.3.7/24 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
					stringifyIPs(addrs),
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()):
				asrt.Equal(
					"1.2.3.4/32 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
					stringifyIPs(addrs),
				)
			case network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()):
				asrt.Equal(
					"25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64",
					stringifyIPs(addrs),
				)
			case network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()):
				asrt.Equal(
					"1.2.3.4/32 25.3.7.9/32 192.168.3.7/24 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
					stringifyIPs(addrs),
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()):
				asrt.Equal("10.0.0.1/8", stringifyIPs(addrs))
			case network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()):
				asrt.Equal("10.0.0.1/8 10.0.0.2/8 192.168.3.7/24", stringifyIPs(addrs))
			}
		},
	)
}

func (suite *NodeAddressSuite) TestSortAlgorithmV2() {
	linkUp := network.NewLinkStatus(network.NamespaceName, "eth0")
	linkUp.TypedSpec().Type = nethelpers.LinkEther
	linkUp.TypedSpec().LinkState = true
	linkUp.TypedSpec().Index = 1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), linkUp))

	linkDown := network.NewLinkStatus(network.NamespaceName, "eth1")
	linkDown.TypedSpec().Type = nethelpers.LinkEther
	linkDown.TypedSpec().LinkState = false
	linkDown.TypedSpec().Index = 2
	suite.Require().NoError(suite.State().Create(suite.Ctx(), linkDown))

	sortAlgorithm := network.NewNodeAddressSortAlgorithm(network.NamespaceName, network.NodeAddressSortAlgorithmID)
	sortAlgorithm.TypedSpec().Algorithm = nethelpers.AddressSortAlgorithmV2
	suite.Require().NoError(suite.State().Create(suite.Ctx(), sortAlgorithm))

	for _, addr := range []string{
		"10.3.4.1/24",
		"10.3.4.5/24",
		"10.3.4.5/32",
		"1.2.3.4/26",
		"192.168.35.11/24",
		"192.168.36.10/24",
		"127.0.0.1/8",
		"::1/128",
		"fd01:cafe::5054:ff:fe1f:c7bd/64",
		"fd01:cafe::f14c:9fa1:8496:557f/128",
	} {
		suite.newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	for _, addr := range []string{"10.0.0.2/8", "192.168.3.7/24"} {
		suite.newAddress(netip.MustParsePrefix(addr), linkDown)
	}

	for _, addr := range []string{"1.2.3.4/26"} { // duplicate with link address: 1.2.3.4
		suite.newExternalAddress(netip.MustParsePrefix(addr))
	}

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressRoutedID,
			network.NodeAddressAccumulativeID,
		},
		func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressDefaultID:
				asrt.Equal("1.2.3.4/26", stringifyIPs(addrs))
			case network.NodeAddressCurrentID, network.NodeAddressRoutedID:
				asrt.Equal(
					"1.2.3.4/26 10.3.4.5/32 10.3.4.1/24 10.3.4.5/24 192.168.35.11/24 192.168.36.10/24 fd01:cafe::f14c:9fa1:8496:557f/128 fd01:cafe::5054:ff:fe1f:c7bd/64",
					stringifyIPs(addrs),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					"1.2.3.4/26 10.0.0.2/8 10.3.4.1/24 10.3.4.5/32 192.168.3.7/24 192.168.35.11/24 192.168.36.10/24 fd01:cafe::5054:ff:fe1f:c7bd/64 fd01:cafe::f14c:9fa1:8496:557f/128",
					stringifyIPs(addrs),
				)
			}
		},
	)
}

func (suite *NodeAddressSuite) TestFilterOverlappingSubnets() {
	linkUp := network.NewLinkStatus(network.NamespaceName, "eth0")
	linkUp.TypedSpec().Type = nethelpers.LinkEther
	linkUp.TypedSpec().LinkState = true
	linkUp.TypedSpec().Index = 1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), linkUp))

	sortAlgorithm := network.NewNodeAddressSortAlgorithm(network.NamespaceName, network.NodeAddressSortAlgorithmID)
	sortAlgorithm.TypedSpec().Algorithm = nethelpers.AddressSortAlgorithmV1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), sortAlgorithm))

	for _, addr := range []string{
		"10.0.0.1/8",
		"10.96.0.2/32",
		"25.3.7.9/32",
	} {
		suite.newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	filter1 := network.NewNodeAddressFilter(network.NamespaceName, "no-k8s")
	filter1.TypedSpec().ExcludeSubnets = []netip.Prefix{netip.MustParsePrefix("10.96.0.0/12")}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), filter1))

	filter2 := network.NewNodeAddressFilter(network.NamespaceName, "only-k8s")
	filter2.TypedSpec().IncludeSubnets = []netip.Prefix{netip.MustParsePrefix("10.96.0.0/12")}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), filter2))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressCurrentID,
			network.NodeAddressRoutedID,
			network.NodeAddressAccumulativeID,
			network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()),
			network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()),
		},
		func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressCurrentID, network.NodeAddressRoutedID, network.NodeAddressAccumulativeID:
				asrt.Equal(
					"10.0.0.1/8 10.96.0.2/32 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()):
				asrt.Equal(
					"10.0.0.1/8 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()):
				asrt.Equal(
					"10.96.0.2/32",
					stringifyIPs(addrs),
				)
			}
		},
	)
}

//nolint:gocyclo
func (suite *NodeAddressSuite) TestDefaultAddressChange() {
	var addressStatusController netctrl.AddressStatusController

	linkUp := network.NewLinkStatus(network.NamespaceName, "eth0")
	linkUp.TypedSpec().Type = nethelpers.LinkEther
	linkUp.TypedSpec().LinkState = true
	linkUp.TypedSpec().Index = 1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), linkUp))

	sortAlgorithm := network.NewNodeAddressSortAlgorithm(network.NamespaceName, network.NodeAddressSortAlgorithmID)
	sortAlgorithm.TypedSpec().Algorithm = nethelpers.AddressSortAlgorithmV1
	suite.Require().NoError(suite.State().Create(suite.Ctx(), sortAlgorithm))

	for _, addr := range []string{
		"10.0.0.5/8",
		"25.3.7.9/32",
		"127.0.0.1/8",
	} {
		suite.newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressAccumulativeID,
		}, func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressDefaultID:
				asrt.Equal("10.0.0.5/8", stringifyIPs(addrs))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					"10.0.0.5/8 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					"10.0.0.5/8 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			}
		},
	)

	// add another address which is "smaller", but default address shouldn't change
	suite.newAddress(netip.MustParsePrefix("1.1.1.1/32"), linkUp)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressAccumulativeID,
		}, func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressDefaultID:
				asrt.Equal("10.0.0.5/8", stringifyIPs(addrs))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					"1.1.1.1/32 10.0.0.5/8 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					"1.1.1.1/32 10.0.0.5/8 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			}
		},
	)

	// remove the previous default address, now default address should change
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		network.NewAddressStatus(network.NamespaceName, network.AddressID(linkUp.Metadata().ID(), netip.MustParsePrefix("10.0.0.5/8"))).Metadata(),
		state.WithDestroyOwner(addressStatusController.Name()),
	))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressAccumulativeID,
		}, func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressDefaultID:
				asrt.Equal("1.1.1.1/32", stringifyIPs(addrs))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					"1.1.1.1/32 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					"1.1.1.1/32 10.0.0.5/8 25.3.7.9/32",
					stringifyIPs(addrs),
				)
			}
		},
	)
}

func TestNodeAddressSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &NodeAddressSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.NodeAddressController{}))
			},
		},
	})
}

func stringifyIPs(ips []netip.Prefix) string {
	return strings.Join(xslices.Map(ips, netip.Prefix.String), " ")
}
