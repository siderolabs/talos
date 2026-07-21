// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"net/netip"

	"github.com/osrg/gobgp/v4/api"
)

// BGPLaunchActivePeerForTest exposes active peer construction for tests.
func BGPLaunchActivePeerForTest(address netip.Addr, peerASN uint32) *api.Peer {
	return bgpLaunchActivePeer(address, peerASN)
}
