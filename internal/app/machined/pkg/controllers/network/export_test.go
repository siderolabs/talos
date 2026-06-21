// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

// Test exports for unexported BGP/route helpers (consumed by the external network_test package).
var (
	RouteSpecForTest       = routeSpec
	AddrFamilyForTest      = addrFamily
	BGPSessionStateForTest = toBGPSessionState
	PathNexthopForTest     = pathNexthop
	BuildMultipathForTest  = buildMultipath
	MultipathEqualForTest  = multipathEqual
)
