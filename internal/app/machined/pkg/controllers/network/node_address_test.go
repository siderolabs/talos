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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
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
					func(a, b netip.Prefix) int {
						return a.Addr().Compare(b.Addr())
					},
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

//nolint:gocyclo
func (suite *NodeAddressSuite) TestFilters() {
	var (
		addressStatusController  netctrl.AddressStatusController
		platformConfigController netctrl.PlatformConfigController
	)

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

	newAddress := func(addr netip.Prefix, link *network.LinkStatus) {
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

	newExternalAddress := func(addr netip.Prefix) {
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

	for _, addr := range []string{
		"10.0.0.1/8",
		"25.3.7.9/32",
		"2001:470:6d:30e:4a62:b3ba:180b:b5b8/64",
		"127.0.0.1/8",
		"fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128",
	} {
		newAddress(netip.MustParsePrefix(addr), linkUp)
	}

	for _, addr := range []string{"10.0.0.2/8", "192.168.3.7/24"} {
		newAddress(netip.MustParsePrefix(addr), linkDown)
	}

	for _, addr := range []string{"1.2.3.4/32", "25.3.7.9/32"} { // duplicate with link address: 25.3.7.9
		newExternalAddress(netip.MustParsePrefix(addr))
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
				asrt.Equal(addrs, ipList("10.0.0.1/8"))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					ipList("1.2.3.4/32 10.0.0.1/8 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
					addrs,
				)
			case network.NodeAddressRoutedID:
				asrt.Equal(
					ipList("10.0.0.1/8 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64"),
					addrs,
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					ipList("1.2.3.4/32 10.0.0.1/8 10.0.0.2/8 25.3.7.9/32 192.168.3.7/24 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
					addrs,
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()):
				asrt.Equal(
					ipList("1.2.3.4/32 25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
					addrs,
				)
			case network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()):
				asrt.Equal(
					ipList("25.3.7.9/32 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64"),
					addrs,
				)
			case network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()):
				asrt.Equal(
					ipList("1.2.3.4/32 25.3.7.9/32 192.168.3.7/24 2001:470:6d:30e:4a62:b3ba:180b:b5b8/64 fdae:41e4:649b:9303:7886:731d:1ce9:4d4/128"),
					addrs,
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()):
				asrt.Equal(addrs, ipList("10.0.0.1/8"))
			case network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()):
				asrt.Equal(addrs, ipList("10.0.0.1/8 10.0.0.2/8 192.168.3.7/24"))
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

	newAddress := func(addr netip.Prefix, link *network.LinkStatus) {
		addressStatus := network.NewAddressStatus(network.NamespaceName, network.AddressID(link.Metadata().ID(), addr))
		addressStatus.TypedSpec().Address = addr
		addressStatus.TypedSpec().LinkName = link.Metadata().ID()
		addressStatus.TypedSpec().LinkIndex = link.TypedSpec().Index
		suite.Require().NoError(
			suite.State().Create(
				suite.Ctx(),
				addressStatus,
			),
		)
	}

	for _, addr := range []string{
		"10.0.0.1/8",
		"10.96.0.2/32",
		"25.3.7.9/32",
	} {
		newAddress(netip.MustParsePrefix(addr), linkUp)
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
					ipList("10.0.0.1/8 10.96.0.2/32 25.3.7.9/32"),
					addrs,
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter1.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter1.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter1.Metadata().ID()):
				asrt.Equal(
					ipList("10.0.0.1/8 25.3.7.9/32"),
					addrs,
				)
			case network.FilteredNodeAddressID(network.NodeAddressCurrentID, filter2.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressRoutedID, filter2.Metadata().ID()),
				network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filter2.Metadata().ID()):
				asrt.Equal(
					ipList("10.96.0.2/32"),
					addrs,
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

	newAddress := func(addr netip.Prefix, link *network.LinkStatus) {
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

	for _, addr := range []string{
		"10.0.0.5/8",
		"25.3.7.9/32",
		"127.0.0.1/8",
	} {
		newAddress(netip.MustParsePrefix(addr), linkUp)
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
				asrt.Equal(addrs, ipList("10.0.0.5/8"))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					addrs,
					ipList("10.0.0.5/8 25.3.7.9/32"),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					addrs,
					ipList("10.0.0.5/8 25.3.7.9/32"),
				)
			}
		},
	)

	// add another address which is "smaller", but default address shouldn't change
	newAddress(netip.MustParsePrefix("1.1.1.1/32"), linkUp)

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		[]resource.ID{
			network.NodeAddressDefaultID,
			network.NodeAddressCurrentID,
			network.NodeAddressAccumulativeID,
		}, func(r *network.NodeAddress, asrt *assert.Assertions) {
			addrs := r.TypedSpec().Addresses

			switch r.Metadata().ID() {
			case network.NodeAddressDefaultID:
				asrt.Equal(addrs, ipList("10.0.0.5/8"))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					addrs,
					ipList("1.1.1.1/32 10.0.0.5/8 25.3.7.9/32"),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					addrs,
					ipList("1.1.1.1/32 10.0.0.5/8 25.3.7.9/32"),
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
				asrt.Equal(addrs, ipList("1.1.1.1/32"))
			case network.NodeAddressCurrentID:
				asrt.Equal(
					addrs,
					ipList("1.1.1.1/32 25.3.7.9/32"),
				)
			case network.NodeAddressAccumulativeID:
				asrt.Equal(
					addrs,
					ipList("1.1.1.1/32 10.0.0.5/8 25.3.7.9/32"),
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

func ipList(ips string) []netip.Prefix {
	var result []netip.Prefix //nolint:prealloc

	for _, ip := range strings.Split(ips, " ") {
		result = append(result, netip.MustParsePrefix(ip))
	}

	return result
}
