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
