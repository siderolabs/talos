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

func (suite *RouteSpecSuite) assertRoute(
	destination netip.Prefix,
	gateway netip.Addr,
	check func(rtnetlink.RouteMessage) error,
) error {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	routes, err := conn.Route.List()
	suite.Require().NoError(err)

	matching := 0

	for _, route := range routes {
		if !route.Attributes.Gateway.Equal(gateway.AsSlice()) {
			continue
		}

		if !(int(route.DstLength) == destination.Bits() || (route.DstLength == 0 && destination.Bits() == -1)) {
			continue
		}

		if !route.Attributes.Dst.Equal(destination.Addr().AsSlice()) {
			continue
		}

		matching++

		if err = check(route); err != nil {
			return retry.ExpectedError(err)
		}
	}

	switch matching {
	case 1:
		return nil
	case 0:
		return retry.ExpectedErrorf("route to %s via %s not found", destination, gateway)
	default:
		return retry.ExpectedErrorf("route to %s via %s found %d matches", destination, gateway, matching)
	}
}

func (suite *RouteSpecSuite) assertNoRoute(destination netip.Prefix, gateway netip.Addr) error {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	routes, err := conn.Route.List()
	suite.Require().NoError(err)

	for _, route := range routes {
		if route.Attributes.Gateway.Equal(gateway.AsSlice()) &&
			(destination.Bits() == int(route.DstLength) || (destination.Bits() == -1 && route.DstLength == 0)) &&
			route.Attributes.Dst.Equal(destination.Addr().AsSlice()) {
			return retry.ExpectedErrorf("route to %s via %s is present", destination, gateway)
		}
	}

	return nil
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

func (suite *RouteSpecSuite) TestDefaultAndInterfaceRoutes() {
	dummyInterface := suite.uniqueDummyInterface()

	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					MTU:  1400,
					Info: &rtnetlink.LinkInfo{
						Kind: "dummy",
					},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	localIP := net.ParseIP("10.28.0.27").To4()

	suite.Require().NoError(
		conn.Address.New(
			&rtnetlink.AddressMessage{
				Family:       unix.AF_INET,
				PrefixLength: 32,
				Scope:        unix.RT_SCOPE_UNIVERSE,
				Index:        uint32(iface.Index),
				Attributes: &rtnetlink.AddressAttributes{
					Address: localIP,
					Local:   localIP,
				},
			},
		),
	)

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

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					MTU:  1500,
					Info: &rtnetlink.LinkInfo{
						Kind: "dummy",
					},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	localIP := net.ParseIP("10.28.0.27").To4()

	suite.Require().NoError(
		conn.Address.New(
			&rtnetlink.AddressMessage{
				Family:       unix.AF_INET,
				PrefixLength: 24,
				Scope:        unix.RT_SCOPE_UNIVERSE,
				Index:        uint32(iface.Index),
				Attributes: &rtnetlink.AddressAttributes{
					Address: localIP,
					Local:   localIP,
				},
			},
		),
	)

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

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					MTU:  1500,
					Info: &rtnetlink.LinkInfo{
						Kind: "dummy",
					},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	suite.Require().NoError(
		conn.Link.Set(
			&rtnetlink.LinkMessage{
				Index: uint32(iface.Index),
				Attributes: &rtnetlink.LinkAttributes{
					Alias: &dummyAlias,
				},
			},
		),
	)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	localIP := net.ParseIP("10.28.0.27").To4()

	suite.Require().NoError(
		conn.Address.New(
			&rtnetlink.AddressMessage{
				Family:       unix.AF_INET,
				PrefixLength: 24,
				Scope:        unix.RT_SCOPE_UNIVERSE,
				Index:        uint32(iface.Index),
				Attributes: &rtnetlink.AddressAttributes{
					Address: localIP,
					Local:   localIP,
				},
			},
		),
	)

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

// assertNoIPv6RouteChurn watches RTNLGRP_IPV6_ROUTE for the given duration and fails on the first
// RTM_DELROUTE for the destination.
//
// A spec the kernel never reports back verbatim makes the controller delete and re-add the route on
// every reconcile, and since the controller watches RTMGRP_IPV6_ROUTE, its own writes wake it up again.
func (suite *RouteSpecSuite) assertNoIPv6RouteChurn(destination netip.Prefix, duration time.Duration) {
	conn, err := rtnetlink.Dial(&netlink.Config{Groups: unix.RTMGRP_IPV6_ROUTE})
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

			if int(route.DstLength) == destination.Bits() && route.Attributes.Dst.Equal(destination.Addr().AsSlice()) {
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

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					Info: &rtnetlink.LinkInfo{Kind: "dummy"},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	localIP := net.ParseIP("2001:db8:1399:6::2").To16()

	suite.Require().NoError(
		conn.Address.New(
			&rtnetlink.AddressMessage{
				Family:       unix.AF_INET6,
				PrefixLength: 64,
				Scope:        unix.RT_SCOPE_UNIVERSE,
				Index:        uint32(iface.Index),
				Attributes: &rtnetlink.AddressAttributes{
					Address: localIP,
					Local:   localIP,
				},
			},
		),
	)

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

	suite.assertNoIPv6RouteChurn(destination, time.Second)

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), route.Metadata()))
	suite.Require().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error { return suite.assertNoRoute(destination, netip.Addr{}) },
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

	suite.Require().NoError(
		conn.Link.New(
			&rtnetlink.LinkMessage{
				Type:   unix.ARPHRD_ETHER,
				Flags:  unix.IFF_UP,
				Change: unix.IFF_UP,
				Attributes: &rtnetlink.LinkAttributes{
					Name: dummyInterface,
					Info: &rtnetlink.LinkInfo{Kind: "dummy"},
				},
			},
		),
	)

	iface, err := net.InterfaceByName(dummyInterface)
	suite.Require().NoError(err)

	defer conn.Link.Delete(uint32(iface.Index)) //nolint:errcheck

	localIP := net.ParseIP("10.28.0.2").To4()

	suite.Require().NoError(
		conn.Address.New(
			&rtnetlink.AddressMessage{
				Family:       unix.AF_INET,
				PrefixLength: 24,
				Scope:        unix.RT_SCOPE_UNIVERSE,
				Index:        uint32(iface.Index),
				Attributes: &rtnetlink.AddressAttributes{
					Address:   localIP,
					Local:     localIP,
					Broadcast: net.ParseIP("10.28.0.255").To4(),
				},
			},
		),
	)

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
					OutIface: uint32(iface.Index),
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
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RouteSpecController{}))
			},
		},
	})
}
