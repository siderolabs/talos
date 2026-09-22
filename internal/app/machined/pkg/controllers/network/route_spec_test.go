// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/netlink"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RouteSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *RouteSpecSuite) uniqueDummyInterface() string {
	return fmt.Sprintf("dummy%02x%02x%02x", rand.Int32()&0xff, rand.Int32()&0xff, rand.Int32()&0xff)
}

// assertSingleRoute lists the kernel routes, expects exactly one to satisfy match, and runs check on it.
//
// The description names the route in the retryable errors, e.g. "route to 10.0.0.0/8 via 10.0.0.1".
func (suite *RouteSpecSuite) assertSingleRoute(
	description string,
	match func(*rtnetlink.RouteMessage) bool,
	check func(rtnetlink.RouteMessage) error,
) error {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	routes, err := conn.Route.List()
	suite.Require().NoError(err)

	matching := 0

	for i := range routes {
		if !match(&routes[i]) {
			continue
		}

		matching++

		if err = check(routes[i]); err != nil {
			return retry.ExpectedError(err)
		}
	}

	switch matching {
	case 1:
		return nil
	case 0:
		return retry.ExpectedErrorf("%s not found", description)
	default:
		return retry.ExpectedErrorf("%s found %d matches", description, matching)
	}
}

// assertRoute finds the single route to the destination via the gateway (any table) and runs check on it.
func (suite *RouteSpecSuite) assertRoute(
	destination netip.Prefix,
	gateway netip.Addr,
	check func(rtnetlink.RouteMessage) error,
) error {
	return suite.assertSingleRoute(
		fmt.Sprintf("route to %s via %s", destination, gateway),
		func(route *rtnetlink.RouteMessage) bool {
			return route.Attributes.Gateway.Equal(gateway.AsSlice()) && netctrl.RouteDestinationMatches(route, destination)
		},
		check,
	)
}

// assertMultipathRoute finds the single route of the family to the destination in the table and runs check on it.
//
// A multipath route carries no top-level gateway, so unlike assertRoute it is identified by its table.
func (suite *RouteSpecSuite) assertMultipathRoute(
	family nethelpers.Family,
	destination netip.Prefix,
	table nethelpers.RoutingTable,
	check func(rtnetlink.RouteMessage) error,
) error {
	return suite.assertSingleRoute(
		fmt.Sprintf("%s route to %s in table %s", family, destination, table),
		func(route *rtnetlink.RouteMessage) bool {
			return route.Family == uint8(family) &&
				nethelpers.RoutingTable(route.Table) == table &&
				netctrl.RouteDestinationMatches(route, destination)
		},
		check,
	)
}

func (suite *RouteSpecSuite) assertNoRoute(destination netip.Prefix, gateway netip.Addr) error {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	routes, err := conn.Route.List()
	suite.Require().NoError(err)

	for i := range routes {
		if routes[i].Attributes.Gateway.Equal(gateway.AsSlice()) && netctrl.RouteDestinationMatches(&routes[i], destination) {
			return retry.ExpectedErrorf("route to %s via %s is present", destination, gateway)
		}
	}

	return nil
}

// createDummyInterface creates an up dummy link with a single address, returning the link index.
//
// The caller is responsible for deleting the link.
func (suite *RouteSpecSuite) createDummyInterface(conn *rtnetlink.Conn, name string, address netip.Prefix) uint32 {
	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Name: name,
					Info: &rtnetlink.LinkInfo{Kind: "dummy"},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(name)
	suite.Require().NoError(err)

	family := uint8(unix.AF_INET)
	if address.Addr().Is6() {
		family = unix.AF_INET6
	}

	suite.Require().NoError(
		conn.Address.New(
			&rtnetlink.AddressMessage{
				Family:       family,
				PrefixLength: uint8(address.Bits()),
				Scope:        unix.RT_SCOPE_UNIVERSE,
				Index:        uint32(iface.Index),
				Attributes: &rtnetlink.AddressAttributes{
					Address: address.Addr().AsSlice(),
					Local:   address.Addr().AsSlice(),
				},
			},
		),
	)

	return uint32(iface.Index)
}

func (suite *RouteSpecSuite) TestLoopback() {
	loopback := network.NewRouteSpec(network.NamespaceName, "loopback")
	*loopback.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("127.0.11.0/24"),
		Gateway:     netip.MustParseAddr("127.0.11.1"),
		OutLinkName: "lo",
		Scope:       nethelpers.ScopeGlobal,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{loopback} {
		suite.Create(res)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(
					netip.MustParsePrefix("127.0.11.0/24"),
					netip.MustParseAddr("127.0.11.1"),
					func(route rtnetlink.RouteMessage) error {
						suite.Assert().EqualValues(0, route.Attributes.Priority)

						return nil
					},
				)
			},
		),
	)

	// teardown the route
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), loopback.Metadata()))

	// torn down address should be removed immediately
	suite.Assert().NoError(
		suite.assertNoRoute(
			netip.MustParsePrefix("127.0.11.0/24"),
			netip.MustParseAddr("127.0.11.1"),
		),
	)
}

func (suite *RouteSpecSuite) TestDefaultRoute() {
	// adding default route with high metric to avoid messing up with the actual default route
	def := network.NewRouteSpec(network.NamespaceName, "default")
	*def.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.Prefix{},
		Gateway:     netip.MustParseAddr("127.0.11.2"),
		Scope:       nethelpers.ScopeGlobal,
		Table:       nethelpers.TableMain,
		OutLinkName: "lo",
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Priority:    1048576,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def} {
		suite.Create(res)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(
					netip.Prefix{}, netip.MustParseAddr("127.0.11.2"), func(route rtnetlink.RouteMessage) error {
						suite.Assert().Nil(route.Attributes.Dst)
						suite.Assert().EqualValues(1048576, route.Attributes.Priority)
						// make sure not extra route metric attributes are set
						suite.Assert().Empty(route.Attributes.Metrics)

						return nil
					},
				)
			},
		),
	)

	// update the route metric and mtu
	ctest.UpdateWithConflicts(suite, def, func(defR *network.RouteSpec) error {
		defR.TypedSpec().MTU = 1700

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(
					netip.Prefix{}, netip.MustParseAddr("127.0.11.2"), func(route rtnetlink.RouteMessage) error {
						suite.Assert().Nil(route.Attributes.Dst)

						if route.Attributes.Metrics == nil || route.Attributes.Metrics.MTU == 0 {
							return fmt.Errorf("route metric wasn't updated: %v", route.Attributes.Metrics)
						}

						suite.Assert().EqualValues(1700, route.Attributes.Metrics.MTU)

						return nil
					},
				)
			},
		),
	)

	// remove mtu and make sure it's unset
	ctest.UpdateWithConflicts(suite, def, func(defR *network.RouteSpec) error {
		defR.TypedSpec().MTU = 0

		return nil
	})

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(
					netip.Prefix{}, netip.MustParseAddr("127.0.11.2"), func(route rtnetlink.RouteMessage) error {
						suite.Assert().Nil(route.Attributes.Dst)

						if route.Attributes.Metrics != nil {
							return retry.ExpectedErrorf("route mtu expected to be empty, got: %d", route.Attributes.Metrics.MTU)
						}

						suite.Assert().Empty(route.Attributes.Metrics)

						return nil
					},
				)
			},
		),
	)

	// teardown the route
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), def.Metadata()))

	// torn down route should be removed immediately
	suite.Assert().NoError(suite.assertNoRoute(netip.Prefix{}, netip.MustParseAddr("127.0.11.2")))
}

func (suite *RouteSpecSuite) TestIPv6DefaultPriorityLifecycle() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	ifaceIndex := suite.createDummyInterface(conn, dummyInterface, netip.MustParsePrefix("2001:db8:1399:1::2/64"))
	defer conn.Link.Delete(ifaceIndex) //nolint:errcheck

	destination := netip.MustParsePrefix("2001:db8:1399:2::/64")
	gateway := netip.MustParseAddr("2001:db8:1399:1::1")
	route := network.NewRouteSpec(network.NamespaceName, "ipv6-default-priority")
	*route.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet6,
		Destination: destination,
		Gateway:     gateway,
		Source:      netip.MustParseAddr("2001:db8:1399:1::2"),
		Table:       nethelpers.TableMain,
		OutLinkName: dummyInterface,
		Protocol:    nethelpers.ProtocolBGP,
		Type:        nethelpers.TypeUnicast,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigOperator,
	}

	suite.Create(route)

	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(destination, gateway, func(message rtnetlink.RouteMessage) error {
					if message.Attributes.Priority != network.DefaultRouteMetric {
						return retry.ExpectedErrorf(
							"route priority expected %d, got %d",
							network.DefaultRouteMetric,
							message.Attributes.Priority,
						)
					}

					return nil
				})
			},
		),
	)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), route.Metadata()))
	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error { return suite.assertNoRoute(destination, gateway) },
		),
	)
}

func (suite *RouteSpecSuite) TestDefaultAndInterfaceRoutes() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	ifaceIndex := suite.createDummyInterface(conn, dummyInterface, netip.MustParsePrefix("10.28.0.27/32"))
	defer conn.Link.Delete(ifaceIndex) //nolint:errcheck

	def := network.NewRouteSpec(network.NamespaceName, "default")
	*def.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.Prefix{},
		Gateway:     netip.MustParseAddr("10.28.0.1"),
		Source:      netip.MustParseAddr("10.28.0.27"),
		Table:       nethelpers.TableMain,
		OutLinkName: dummyInterface,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Priority:    1048576,
		ConfigLayer: network.ConfigMachineConfiguration,
	}
	def.TypedSpec().Normalize()

	host := network.NewRouteSpec(network.NamespaceName, "aninterface")
	*host.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("10.28.0.1/32"),
		Gateway:     netip.MustParseAddr("0.0.0.0"),
		Source:      netip.MustParseAddr("10.28.0.27"),
		Table:       nethelpers.TableMain,
		OutLinkName: dummyInterface,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Priority:    1048576,
		ConfigLayer: network.ConfigMachineConfiguration,
	}
	host.TypedSpec().Normalize()

	for _, res := range []resource.Resource{def, host} {
		suite.Create(res)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				if err := suite.assertRoute(
					netip.Prefix{}, netip.MustParseAddr("10.28.0.1"), func(route rtnetlink.RouteMessage) error {
						suite.Assert().Nil(route.Attributes.Dst)
						suite.Assert().EqualValues(1048576, route.Attributes.Priority)

						return nil
					},
				); err != nil {
					return err
				}

				return suite.assertRoute(
					netip.MustParsePrefix("10.28.0.1/32"), netip.Addr{}, func(route rtnetlink.RouteMessage) error {
						suite.Assert().Nil(route.Attributes.Gateway)
						suite.Assert().EqualValues(1048576, route.Attributes.Priority)

						return nil
					},
				)
			},
		),
	)

	// teardown the routes
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), def.Metadata()))
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), host.Metadata()))

	// torn down route should be removed immediately
	suite.Assert().NoError(suite.assertNoRoute(netip.Prefix{}, netip.MustParseAddr("10.28.0.1")))
	suite.Assert().NoError(suite.assertNoRoute(netip.MustParsePrefix("10.28.0.1/32"), netip.Addr{}))
}

func (suite *RouteSpecSuite) TestLinkLocalRoute() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	ifaceIndex := suite.createDummyInterface(conn, dummyInterface, netip.MustParsePrefix("10.28.0.27/24"))
	defer conn.Link.Delete(ifaceIndex) //nolint:errcheck

	ll := network.NewRouteSpec(network.NamespaceName, "ll")
	*ll.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("169.254.169.254/32"),
		Gateway:     netip.MustParseAddr("10.28.0.1"),
		Source:      netip.MustParseAddr("10.28.0.27"),
		Table:       nethelpers.TableMain,
		OutLinkName: dummyInterface,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Priority:    1048576,
		ConfigLayer: network.ConfigMachineConfiguration,
	}
	ll.TypedSpec().Normalize()

	for _, res := range []resource.Resource{ll} {
		suite.Create(res)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(
					netip.MustParsePrefix("169.254.169.254/32"),
					netip.MustParseAddr("10.28.0.1"),
					func(route rtnetlink.RouteMessage) error {
						suite.Assert().EqualValues(1048576, route.Attributes.Priority)

						return nil
					},
				)
			},
		),
	)

	// teardown the routes
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), ll.Metadata()))

	// torn down route should be removed immediately
	suite.Assert().NoError(
		suite.assertNoRoute(
			netip.MustParsePrefix("169.254.169.254/32"),
			netip.MustParseAddr("10.28.0.1"),
		),
	)
}

func (suite *RouteSpecSuite) TestLinkLocalRouteAlias() {
	dummyInterface := suite.uniqueDummyInterface()
	dummyAlias := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	ifaceIndex := suite.createDummyInterface(conn, dummyInterface, netip.MustParsePrefix("10.28.0.27/24"))

	suite.Require().NoError(
		conn.Link.Set(
			&rtnetlink.LinkMessage{
				Index: ifaceIndex,
				Attributes: &rtnetlink.LinkAttributes{
					Alias: &dummyAlias,
				},
			},
		),
	)

	defer conn.Link.Delete(ifaceIndex) //nolint:errcheck

	ll := network.NewRouteSpec(network.NamespaceName, "ll")
	*ll.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("169.254.169.254/32"),
		Gateway:     netip.MustParseAddr("10.28.0.1"),
		Source:      netip.MustParseAddr("10.28.0.27"),
		Table:       nethelpers.TableMain,
		OutLinkName: dummyAlias, // using alias name instead of the actual interface name
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Priority:    1048576,
		ConfigLayer: network.ConfigMachineConfiguration,
	}
	ll.TypedSpec().Normalize()

	for _, res := range []resource.Resource{ll} {
		suite.Create(res)
	}

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(
					netip.MustParsePrefix("169.254.169.254/32"),
					netip.MustParseAddr("10.28.0.1"),
					func(route rtnetlink.RouteMessage) error {
						suite.Assert().EqualValues(1048576, route.Attributes.Priority)

						return nil
					},
				)
			},
		),
	)

	// teardown the routes
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), ll.Metadata()))

	// torn down route should be removed immediately
	suite.Assert().NoError(
		suite.assertNoRoute(
			netip.MustParsePrefix("169.254.169.254/32"),
			netip.MustParseAddr("10.28.0.1"),
		),
	)
}

// assertNoRouteChurn watches route changes of the given family for the given duration and fails on
// the first RTM_DELROUTE for the destination.
//
// A spec the kernel never reports back verbatim makes the controller delete and re-add the route on
// every reconcile, and since the controller watches the route groups, its own writes wake it up again.
func (suite *RouteSpecSuite) assertNoRouteChurn(family nethelpers.Family, destination netip.Prefix, duration time.Duration) {
	group := uint32(unix.RTMGRP_IPV4_ROUTE)

	if family == nethelpers.FamilyInet6 {
		group = unix.RTMGRP_IPV6_ROUTE
	}

	conn, err := rtnetlink.Dial(&netlink.Config{Groups: group})
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	suite.Require().NoError(conn.SetReadDeadline(time.Now().Add(duration)))

	for {
		rtmsgs, msgs, err := conn.Receive()
		if err != nil {
			suite.Require().ErrorIs(err, os.ErrDeadlineExceeded)

			return
		}

		for i, msg := range msgs {
			if msg.Header.Type != unix.RTM_DELROUTE {
				continue
			}

			route, ok := rtmsgs[i].(*rtnetlink.RouteMessage)
			if !ok {
				continue
			}

			if netctrl.RouteDestinationMatches(route, destination) {
				suite.Require().Failf(
					"route churn",
					"unexpected RTM_DELROUTE for %s: the route is being rewritten on every reconcile",
					destination,
				)
			}
		}
	}
}

func (suite *RouteSpecSuite) TestIPv6GatewaylessRoute() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	ifaceIndex := suite.createDummyInterface(conn, dummyInterface, netip.MustParsePrefix("2001:db8:1399:6::2/64"))
	defer conn.Link.Delete(ifaceIndex) //nolint:errcheck

	destination := netip.MustParsePrefix("2001:db8:1399:7::/64")

	route := network.NewRouteSpec(network.NamespaceName, "ipv6-gatewayless")
	*route.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet6,
		Destination: destination,
		OutLinkName: dummyInterface,
		Table:       nethelpers.TableMain,
		Priority:    network.DefaultRouteMetric,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		// Normalize() assigns link scope to any route with a destination and no gateway, regardless of family
		Scope:       nethelpers.ScopeLink,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(route)

	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(destination, netip.Addr{}, func(message rtnetlink.RouteMessage) error {
					// the IPv6 FIB doesn't store the scope, so a link-scoped spec is reported back as global
					if message.Scope != uint8(nethelpers.ScopeGlobal) {
						return retry.ExpectedErrorf(
							"route scope expected %d, got %d",
							nethelpers.ScopeGlobal,
							message.Scope,
						)
					}

					return nil
				})
			},
		),
	)

	suite.assertNoRouteChurn(nethelpers.FamilyInet6, destination, time.Second)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), route.Metadata()))
	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error { return suite.assertNoRoute(destination, netip.Addr{}) },
		),
	)
}

// TestMultipathRouteNumberedNextHops covers ECMP next-hops which carry no out-link, as learned from
// numbered (non-link-local) BGP peers: the kernel resolves the egress device from the gateway and
// reports it back, and the controller must accept that instead of rewriting the route forever.
//
// See https://github.com/siderolabs/talos/issues/14416.
func (suite *RouteSpecSuite) TestMultipathRouteNumberedNextHops() {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	firstIndex := suite.createDummyInterface(conn, suite.uniqueDummyInterface(), netip.MustParsePrefix("192.0.2.1/31"))
	defer conn.Link.Delete(firstIndex) //nolint:errcheck

	secondIndex := suite.createDummyInterface(conn, suite.uniqueDummyInterface(), netip.MustParsePrefix("192.0.2.3/31"))
	defer conn.Link.Delete(secondIndex) //nolint:errcheck

	gateways := []netip.Addr{netip.MustParseAddr("192.0.2.0"), netip.MustParseAddr("192.0.2.2")}
	indices := []uint32{firstIndex, secondIndex}

	destination := netip.MustParsePrefix("0.0.0.0/0")
	// a dedicated routing table keeps the test default route away from the host routing
	table := nethelpers.RoutingTable(200)

	route := network.NewRouteSpec(network.NamespaceName, "bgp-ecmp-numbered")
	*route.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: destination,
		// no OutLinkName: a numbered peer doesn't resolve to an interface, the kernel picks one
		NextHops: []network.RouteNextHop{
			{Gateway: gateways[0]},
			{Gateway: gateways[1]},
		},
		Table:       table,
		Protocol:    nethelpers.ProtocolBGP,
		Type:        nethelpers.TypeUnicast,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigOperator,
	}

	suite.Create(route)

	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertMultipathRoute(nethelpers.FamilyInet4, destination, table, func(message rtnetlink.RouteMessage) error {
					if len(message.Attributes.Multipath) != len(gateways) {
						return retry.ExpectedErrorf(
							"expected %d next-hops, got %d",
							len(gateways),
							len(message.Attributes.Multipath),
						)
					}

					for i, hop := range message.Attributes.Multipath {
						if !hop.Gateway.Equal(gateways[i].AsSlice()) {
							return retry.ExpectedErrorf("next-hop %d gateway expected %s, got %s", i, gateways[i], hop.Gateway)
						}

						// the kernel resolves the egress device from the gateway
						if hop.Hop.IfIndex != indices[i] {
							return retry.ExpectedErrorf("next-hop %d link expected %d, got %d", i, indices[i], hop.Hop.IfIndex)
						}
					}

					return nil
				})
			},
		),
	)

	suite.assertNoRouteChurn(nethelpers.FamilyInet4, destination, time.Second)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), route.Metadata()))
	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				err := suite.assertMultipathRoute(nethelpers.FamilyInet4, destination, table, func(rtnetlink.RouteMessage) error { return nil })
				if err == nil {
					return retry.ExpectedErrorf("route to %s in table %s is still present", destination, table)
				}

				return nil
			},
		),
	)
}

// TestIPv4RouteScopeMismatch covers the other side of routeScopeMatches: for IPv4 the kernel does
// store the scope, so a route already present with the wrong one must be rewritten.
func (suite *RouteSpecSuite) TestIPv4RouteScopeMismatch() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	ifaceIndex := suite.createDummyInterface(conn, dummyInterface, netip.MustParsePrefix("10.28.0.2/24"))
	defer conn.Link.Delete(ifaceIndex) //nolint:errcheck

	destination := netip.MustParsePrefix("10.29.0.0/24")

	// install the route out of band with a global scope: everything else matches the spec below, so
	// the scope is the only reason for the controller to rewrite it
	suite.Require().NoError(
		conn.Route.Add(
			&rtnetlink.RouteMessage{
				Family:    unix.AF_INET,
				DstLength: 24,
				Protocol:  unix.RTPROT_STATIC,
				Scope:     unix.RT_SCOPE_UNIVERSE,
				Type:      unix.RTN_UNICAST,
				Attributes: rtnetlink.RouteAttributes{
					Dst:      destination.Addr().AsSlice(),
					OutIface: ifaceIndex,
					Priority: network.DefaultRouteMetric,
					Table:    unix.RT_TABLE_MAIN,
				},
			},
		),
	)

	route := network.NewRouteSpec(network.NamespaceName, "ipv4-scope-mismatch")
	*route.TypedSpec() = network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: destination,
		OutLinkName: dummyInterface,
		Table:       nethelpers.TableMain,
		Priority:    network.DefaultRouteMetric,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Scope:       nethelpers.ScopeLink,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	suite.Create(route)

	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertRoute(destination, netip.Addr{}, func(message rtnetlink.RouteMessage) error {
					if message.Scope != uint8(nethelpers.ScopeLink) {
						return retry.ExpectedErrorf(
							"route scope expected %d, got %d",
							nethelpers.ScopeLink,
							message.Scope,
						)
					}

					return nil
				})
			},
		),
	)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), route.Metadata()))
	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error { return suite.assertNoRoute(destination, netip.Addr{}) },
		),
	)
}

func TestRouteScopeMatches(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		family   nethelpers.Family
		actual   nethelpers.Scope
		expected nethelpers.Scope
		matches  bool
	}{
		{
			name:     "inet4 equal",
			family:   nethelpers.FamilyInet4,
			actual:   nethelpers.ScopeLink,
			expected: nethelpers.ScopeLink,
			matches:  true,
		},
		{
			name:     "inet4 mismatch",
			family:   nethelpers.FamilyInet4,
			actual:   nethelpers.ScopeGlobal,
			expected: nethelpers.ScopeLink,
			matches:  false,
		},
		{
			// the kernel reports every IPv6 route as global, whatever scope the spec asked for
			name:     "inet6 link spec reported as global",
			family:   nethelpers.FamilyInet6,
			actual:   nethelpers.ScopeGlobal,
			expected: nethelpers.ScopeLink,
			matches:  true,
		},
		{
			name:     "inet6 equal",
			family:   nethelpers.FamilyInet6,
			actual:   nethelpers.ScopeGlobal,
			expected: nethelpers.ScopeGlobal,
			matches:  true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.matches, netctrl.RouteScopeMatches(
				uint8(test.actual),
				&network.RouteSpecSpec{Family: test.family, Scope: test.expected},
			))
		})
	}
}

func TestRouteSpecSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, &RouteSpecSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RouteSpecController{}))
		},
	})
}
