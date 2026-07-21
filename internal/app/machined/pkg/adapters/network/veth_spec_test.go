// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/netlink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestVethSpecEncode(t *testing.T) {
	t.Parallel()

	data, err := networkadapter.VethSpec(&network.VethSpec{PeerName: "veth-peer"}).Encode()
	require.NoError(t, err)

	decoder, err := netlink.NewAttributeDecoder(data)
	require.NoError(t, err)

	require.True(t, decoder.Next())
	assert.EqualValues(t, 1, decoder.Type())

	var peer rtnetlink.LinkMessage
	require.NoError(t, peer.UnmarshalBinary(decoder.Bytes()))
	require.NoError(t, decoder.Err())
	assert.Equal(t, "veth-peer", peer.Attributes.Name)
}

func TestVethSpecEncodeRequiresPeer(t *testing.T) {
	t.Parallel()

	_, err := networkadapter.VethSpec(&network.VethSpec{}).Encode()
	assert.EqualError(t, err, "veth peer name must be specified")
}
