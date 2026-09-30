// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"time"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/netlink"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sys/unix"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TestNextHopSetTransition replaces a single next-hop route with an ECMP route to the same destination
// (and back), the way the merge controller does it when a BGP path is added or lost: the old RouteSpec
// (its ID carries the gateway) is torn down and a new one (no gateway in the ID) is created right away.
//
// The route must never be missing from the kernel for longer than a reconcile.
//
//nolint:gocyclo
func (suite *RouteSpecSuite) TestNextHopSetTransition() {
	conn, err := rtnetlink.Dial(nil)
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	first, second := suite.uniqueDummyInterface(), suite.uniqueDummyInterface()

	firstIndex := suite.createDummyInterface(conn, first, netip.MustParsePrefix("192.0.2.1/31"))
	defer conn.Link.Delete(firstIndex) //nolint:errcheck

	secondIndex := suite.createDummyInterface(conn, second, netip.MustParsePrefix("192.0.2.3/31"))
	defer conn.Link.Delete(secondIndex) //nolint:errcheck

	destination := netip.MustParsePrefix("0.0.0.0/0")
	table := nethelpers.RoutingTable(201)

	base := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: destination,
		Table:       table,
		Protocol:    nethelpers.ProtocolBGP,
		Type:        nethelpers.TypeUnicast,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigOperator,
	}

	single := func() *network.RouteSpec {
		gw := netip.MustParseAddr("192.0.2.0")
		r := network.NewRouteSpec(network.NamespaceName, network.RouteID(table, base.Family, destination, gw, 0, ""))
		*r.TypedSpec() = base
		r.TypedSpec().Gateway = gw
		r.TypedSpec().OutLinkName = first

		return r
	}

	multi := func() *network.RouteSpec {
		r := network.NewRouteSpec(network.NamespaceName, network.RouteID(table, base.Family, destination, netip.Addr{}, 0, ""))
		*r.TypedSpec() = base
		r.TypedSpec().NextHops = []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("192.0.2.0"), OutLinkName: first},
			{Gateway: netip.MustParseAddr("192.0.2.2"), OutLinkName: second},
		}

		return r
	}

	hops := func(want int) func() error {
		return func() error {
			return suite.assertMultipathRoute(nethelpers.FamilyInet4, destination, table, func(m rtnetlink.RouteMessage) error {
				got := len(m.Attributes.Multipath)
				if got == 0 {
					got = 1
				}

				if got != want {
					return retry.ExpectedErrorf("expected %d next-hops, got %d", want, got)
				}

				return nil
			})
		}
	}

	current := single()
	suite.Create(current)
	suite.Require().NoError(retry.Constant(3*time.Second, retry.WithUnits(50*time.Millisecond)).Retry(hops(1)))

	// watch the destination in the test table: the longest time it is absent from the kernel
	watch, err := rtnetlink.Dial(&netlink.Config{Groups: unix.RTMGRP_IPV4_ROUTE})
	suite.Require().NoError(err)

	defer watch.Close() //nolint:errcheck

	longestGap := make(chan time.Duration, 1)

	go func() {
		var (
			longest  time.Duration
			goneAt   time.Time
			isAbsent bool
		)

		for {
			rtmsgs, msgs, err := watch.Receive()
			if err != nil {
				longestGap <- longest

				return
			}

			for i, msg := range msgs {
				route, ok := rtmsgs[i].(*rtnetlink.RouteMessage)
				if !ok || nethelpers.RoutingTable(route.Table) != table || !netctrl.RouteDestinationMatches(route, destination) {
					continue
				}

				switch msg.Header.Type { //nolint:exhaustive
				case unix.RTM_DELROUTE:
					goneAt, isAbsent = time.Now(), true
				case unix.RTM_NEWROUTE:
					if isAbsent {
						longest = max(longest, time.Since(goneAt))
						isAbsent = false
					}
				}
			}
		}
	}()

	for i := range 10 {
		next, want := multi(), 2
		if i%2 == 1 {
			next, want = single(), 1
		}

		// same order as the merge controller: tear the stale spec down, then create the new one
		_, err = suite.State().Teardown(suite.Ctx(), current.Metadata())
		suite.Require().NoError(err)
		suite.Create(next)
		suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), current.Metadata()))
		suite.Require().NoError(retry.Constant(3*time.Second, retry.WithUnits(10*time.Millisecond)).Retry(hops(want)))

		current = next
	}

	suite.Require().NoError(watch.SetReadDeadline(time.Now()))

	gap := <-longestGap
	suite.T().Logf("longest time without a route to %s: %s", destination, gap)
	suite.Assert().Less(gap, 50*time.Millisecond, "route was missing while switching between single and multipath next hops")

	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), current.Metadata()))
}
