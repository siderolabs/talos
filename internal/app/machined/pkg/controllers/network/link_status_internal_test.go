// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"

	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestVethPeerName(t *testing.T) {
	t.Parallel()

	link := func(index, peerIndex uint32, name, kind string) rtnetlink.LinkMessage {
		return rtnetlink.LinkMessage{
			Index: index,
			Attributes: &rtnetlink.LinkAttributes{
				Name: name,
				Type: peerIndex,
				Info: &rtnetlink.LinkInfo{Kind: kind},
			},
		}
	}

	current := link(10, 20, "veth0", networkres.LinkKindVeth)

	assert.Equal(t, "veth1", vethPeerName([]rtnetlink.LinkMessage{
		current,
		link(20, 10, "veth1", networkres.LinkKindVeth),
	}, current))

	assert.Empty(t, vethPeerName([]rtnetlink.LinkMessage{
		current,
		link(20, 10, "dummy0", "dummy"),
	}, current), "an unrelated local interface can reuse a cross-namespace peer index")

	assert.Empty(t, vethPeerName([]rtnetlink.LinkMessage{
		current,
		link(20, 30, "veth1", networkres.LinkKindVeth),
	}, current), "a same-namespace peer must point back to the current interface")

	assert.Empty(t, vethPeerName([]rtnetlink.LinkMessage{current}, current))
}
