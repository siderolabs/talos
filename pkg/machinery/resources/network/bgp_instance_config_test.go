// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBGPInstanceConfigDeepCopy(t *testing.T) {
	t.Parallel()

	spec := network.BGPInstanceConfigSpec{
		AdvertiseLinks: []string{"dummy0"},
		Neighbors: []network.BGPNeighborConfigSpec{{
			Link: "eth0",
			BFD:  &network.BGPBFDConfigSpec{TransmitInterval: time.Second},
		}},
	}

	clone := spec.DeepCopy()
	clone.AdvertiseLinks[0] = "dummy1"
	clone.Neighbors[0].Link = "eth1"
	clone.Neighbors[0].BFD.TransmitInterval = 2 * time.Second

	assert.Equal(t, "dummy0", spec.AdvertiseLinks[0])
	assert.Equal(t, "eth0", spec.Neighbors[0].Link)
	assert.Equal(t, time.Second, spec.Neighbors[0].BFD.TransmitInterval)
}

func TestBGPInstanceConfigProtobufRoundTrip(t *testing.T) {
	t.Parallel()

	config := network.NewBGPInstanceConfig("fabric")
	*config.TypedSpec() = network.BGPInstanceConfigSpec{
		LocalASN:       65001,
		RouterID:       netip.MustParseAddr("192.0.2.1"),
		RouteSource:    netip.MustParseAddr("192.0.2.2"),
		AdvertiseLinks: []string{"dummy0"},
		Multipath:      true,
		MaxPaths:       8,
		VRF:            "vrf-blue",
		VRFTable:       88,
		Neighbors: []network.BGPNeighborConfigSpec{
			{
				Link:     "eth0",
				PeerASN:  65002,
				LocalASN: 65004,
				Passive:  true,
				HoldTime: 90 * time.Second,
				BFD: &network.BGPBFDConfigSpec{
					TransmitInterval: 300 * time.Millisecond,
					ReceiveInterval:  400 * time.Millisecond,
					DetectMultiplier: 3,
				},
			},
			{
				Address: netip.MustParseAddr("198.51.100.1"),
				PeerASN: 65003,
			},
		},
	}

	protoResource, err := protobuf.FromResource(config)
	require.NoError(t, err)

	marshaled, err := protoResource.Marshal()
	require.NoError(t, err)

	protoResource, err = protobuf.Unmarshal(marshaled)
	require.NoError(t, err)

	roundTripped, err := protobuf.UnmarshalResource(protoResource)
	require.NoError(t, err)
	require.True(t, resource.Equal(config, roundTripped))
}
