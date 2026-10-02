// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"math/rand/v2"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RouteMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *RouteMergeSuite) assertRoutes(requiredIDs []string, check func(*network.RouteSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *RouteMergeSuite) assertNoRoute(id string) {
	ctest.AssertNoResource[*network.RouteSpec](suite, id)
}

func (suite *RouteMergeSuite) TestMerge() {
	cmdline := network.NewRouteSpec(network.ConfigNamespaceName, "cmdline/inet4//50")
	*cmdline.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigCmdline,
	}

	dhcp := network.NewRouteSpec(network.ConfigNamespaceName, "dhcp/inet4//50")
	*dhcp.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewRouteSpec(network.ConfigNamespaceName, "configuration/inet4/10.0.0.35/32/1024")
	*static.TypedSpec() = network.RouteSpecSpec{
		Destination: netip.MustParsePrefix("10.0.0.35/32"),
		Gateway:     netip.MustParseAddr("10.0.0.34"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    1024,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{cmdline, dhcp, static} {
		suite.Create(res)
	}

	suite.assertRoutes(
		[]string{
			"inet4//50",
			"inet4/10.0.0.35/32/1024",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())

			switch r.Metadata().ID() {
			case "inet4//50":
				asrt.Equal(*dhcp.TypedSpec(), *r.TypedSpec())
			case "inet4/10.0.0.35/32/1024":
				asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	suite.Destroy(dhcp)

	suite.assertRoutes(
		[]string{
			"inet4//50",
			"inet4/10.0.0.35/32/1024",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())

			switch r.Metadata().ID() {
			case "inet4//50":
				asrt.Equal(*cmdline.TypedSpec(), *r.TypedSpec())
			case "inet4/10.0.0.35/32/1024":
				asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	suite.Destroy(static)

	suite.assertNoRoute("inet4/10.0.0.35/32/1024")
}

func (suite *RouteMergeSuite) TestMergeSameKey() {
	// the kernel keeps a single route per table, destination and priority, so the routes which differ only in
	// next-hops are merged into a single route, higher layer wins
	bgp := network.NewRouteSpec(network.ConfigNamespaceName, "bgp/fabric/inet4//0")
	*bgp.TypedSpec() = network.RouteSpecSpec{
		Destination: netip.MustParsePrefix("0.0.0.0/0"),
		NextHops: []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("fe80::1"), OutLinkName: "eth0"},
			{Gateway: netip.MustParseAddr("fe80::2"), OutLinkName: "eth1"},
		},
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolBGP,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewRouteSpec(network.ConfigNamespaceName, "configuration/inet4//0")
	*static.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.0.0.1"),
		OutLinkName: "eth2",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	// IPv6 routes without a priority get the default metric in the kernel
	ipv6Default := network.NewRouteSpec(network.ConfigNamespaceName, "dhcp/inet6//1024")
	*ipv6Default.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("fe80::3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet6,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    network.DefaultRouteMetric,
		ConfigLayer: network.ConfigOperator,
	}

	ipv6NoPriority := network.NewRouteSpec(network.ConfigNamespaceName, "configuration/inet6//0")
	*ipv6NoPriority.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("fe80::4"),
		OutLinkName: "eth1",
		Family:      nethelpers.FamilyInet6,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{bgp, static, ipv6Default, ipv6NoPriority} {
		suite.Create(res)
	}

	suite.assertRoutes(
		[]string{
			"inet4//0",
			"inet6//1024",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "inet4//0":
				asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			case "inet6//1024":
				asrt.Equal(*ipv6NoPriority.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	suite.Destroy(static)

	suite.assertRoutes(
		[]string{
			"inet4//0",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
			asrt.Equal(*bgp.TypedSpec(), *r.TypedSpec())
		},
	)

	// next-hops change in place, the route keeps its ID
	bgp.TypedSpec().NextHops = nil
	bgp.TypedSpec().Gateway = netip.MustParseAddr("fe80::1")
	bgp.TypedSpec().OutLinkName = "eth0"
	suite.Update(bgp)

	suite.assertRoutes(
		[]string{
			"inet4//0",
		}, func(r *network.RouteSpec, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
			asrt.Equal(*bgp.TypedSpec(), *r.TypedSpec())
		},
	)
}

func testMergeFlapping[R rtestutils.ResourceWithRD](suite *ctest.DefaultSuite, resources []R, outputID string, mergedResource R) {
	var zeroR R

	outputMetadata := resource.NewMetadata(
		zeroR.ResourceDefinition().DefaultNamespace,
		zeroR.ResourceDefinition().Type,
		outputID,
		resource.VersionUndefined,
	)

	for range 30 {
		// pick a set of input resources to create, each bit in the choice represents a resource
		choice := rand.IntN(1 << len(resources))

		if choice == 0 {
			continue
		}

		for idx, res := range resources {
			if choice&(1<<idx) != 0 {
				suite.Create(res)
			}
		}

		// wait for output to be created
		ctest.AssertResources(suite, []string{outputID}, func(r R, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
		})

		// put a finalizer on the output resource
		suite.Require().NoError(suite.State().AddFinalizer(
			suite.Ctx(),
			outputMetadata,
			"foo",
		))

		for idx, res := range resources {
			if choice&(1<<idx) != 0 {
				suite.Destroy(res)
			}
		}

		// wait for output to be torn down
		ctest.AssertResources(suite, []string{outputID}, func(r R, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
		})

		// remove a finalizer
		suite.Require().NoError(suite.State().RemoveFinalizer(
			suite.Ctx(),
			outputMetadata,
			"foo",
		))
	}

	// create all resources
	for _, res := range resources {
		suite.Create(res)
	}

	ctest.AssertResources(
		suite,
		[]string{
			outputID,
		}, func(r R, asrt *assert.Assertions) {
			asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
			asrt.Equal(mergedResource.Spec(), r.Spec())
		},
	)
}

//nolint:gocyclo
func (suite *RouteMergeSuite) TestMergeFlapping() {
	// simulate two conflicting default route definitions which are getting removed/added constantly
	cmdline := network.NewRouteSpec(network.ConfigNamespaceName, "cmdline/inet4//50")
	*cmdline.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigCmdline,
	}

	dhcp := network.NewRouteSpec(network.ConfigNamespaceName, "dhcp/inet4//50")
	*dhcp.TypedSpec() = network.RouteSpecSpec{
		Gateway:     netip.MustParseAddr("10.5.0.3"),
		OutLinkName: "eth1",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Type:        nethelpers.TypeUnicast,
		Table:       nethelpers.TableMain,
		Priority:    50,
		ConfigLayer: network.ConfigOperator,
	}

	testMergeFlapping(&suite.DefaultSuite, []*network.RouteSpec{cmdline, dhcp}, "inet4//50", dhcp)
}

func TestRouteMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RouteMergeSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			s.Require().NoError(s.Runtime().RegisterController(netctrl.NewRouteMergeController()))
		},
	})
}
