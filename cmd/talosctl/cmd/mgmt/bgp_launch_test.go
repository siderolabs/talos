// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt_test

import (
	"net/netip"
	"testing"

	"github.com/osrg/gobgp/v4/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt"
)

func TestBGPLaunchActivePeer(t *testing.T) {
	t.Parallel()

	peer := mgmt.BGPLaunchActivePeerForTest(netip.MustParseAddr("192.0.2.2"), 65001)

	assert.Equal(t, "192.0.2.2", peer.GetConf().GetNeighborAddress())
	assert.Equal(t, uint32(65001), peer.GetConf().GetPeerAsn())
	assert.Equal(t, uint64(1), peer.GetTimers().GetConfig().GetConnectRetry())
	require.Len(t, peer.GetAfiSafis(), 2)
	assert.Equal(t, api.Family_AFI_IP, peer.GetAfiSafis()[0].GetConfig().GetFamily().GetAfi())
	assert.Equal(t, api.Family_AFI_IP6, peer.GetAfiSafis()[1].GetConfig().GetFamily().GetAfi())
}

func TestBGPLaunchHostRouteEligible(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		prefix string
		valid  bool
	}{
		{prefix: "172.16.0.2/32", valid: true},
		{prefix: "198.51.100.100/32", valid: true},
		{prefix: "2001:db8::1/128", valid: true},
		{prefix: "10.244.0.0/24", valid: false},
		{prefix: "2001:db8::/64", valid: false},
	} {
		t.Run(tc.prefix, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.valid, mgmt.BGPLaunchHostRouteEligibleForTest(netip.MustParsePrefix(tc.prefix)))
		})
	}
}
