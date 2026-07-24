// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package mgmt //nolint:testpackage // exports Linux-only route ownership helpers to the external test package

import (
	"net/netip"

	"github.com/jsimonetti/rtnetlink/v2"
)

// BGPLaunchOwnedRouteInventoryForTest exposes fabric route adoption for tests.
func BGPLaunchOwnedRouteInventoryForTest(
	routes []rtnetlink.RouteMessage,
	ifaces map[uint32]struct{},
) map[netip.Prefix]rtnetlink.RouteMessage {
	return fabricOwnedRouteInventory(routes, ifaces)
}

// BGPLaunchOwnedRoutePrefixForTest exposes fabric route ownership checks for tests.
func BGPLaunchOwnedRoutePrefixForTest(
	route rtnetlink.RouteMessage,
	ifaces map[uint32]struct{},
) (netip.Prefix, bool) {
	return fabricOwnedRoutePrefix(route, ifaces)
}
