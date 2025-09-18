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
	"github.com/siderolabs/go-retry/retry"
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
