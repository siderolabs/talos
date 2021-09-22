// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net"
	"testing"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/network"
)

func TestVLANSpec(t *testing.T) {
	spec := network.VLANSpec{
		VID:      25,
		Protocol: nethelpers.VLANProtocol8021AD,
	}

	b, err := spec.Encode()
	require.NoError(t, err)

	var decodedSpec network.VLANSpec

	require.NoError(t, decodedSpec.Decode(b))

	require.Equal(t, spec, decodedSpec)
}

func TestBondMasterSpec(t *testing.T) {
	spec := network.BondMasterSpec{
		Mode:      nethelpers.BondModeActiveBackup,
		MIIMon:    100,
		UpDelay:   200,
		DownDelay: 300,
	}

	b, err := spec.Encode()
	require.NoError(t, err)

	var decodedSpec network.BondMasterSpec

	require.NoError(t, decodedSpec.Decode(b))

	require.Equal(t, spec, decodedSpec)
}

func TestWireguardPeer(t *testing.T) {
	key1, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	key2, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	peer1 := network.WireguardPeer{
		PublicKey:                   key1.PublicKey().String(),
		Endpoint:                    "127.0.0.1:1000",
		PersistentKeepaliveInterval: 10 * time.Second,
		AllowedIPs: []netaddr.IPPrefix{
			netaddr.MustParseIPPrefix("10.2.0.0/16"),
			netaddr.MustParseIPPrefix("10.2.0.0/24"),
		},
	}

	peer2 := network.WireguardPeer{
		PublicKey: key2.PublicKey().String(),
		Endpoint:  "127.0.0.1:2000",
		AllowedIPs: []netaddr.IPPrefix{
			netaddr.MustParseIPPrefix("10.2.0.0/15"),
			netaddr.MustParseIPPrefix("10.3.0.0/28"),
		},
	}

	peer1_1 := network.WireguardPeer{
		PublicKey:                   key1.PublicKey().String(),
		Endpoint:                    "127.0.0.1:1000",
		PersistentKeepaliveInterval: 10 * time.Second,
		AllowedIPs: []netaddr.IPPrefix{
			netaddr.MustParseIPPrefix("10.2.0.0/15"),
			netaddr.MustParseIPPrefix("10.3.0.0/28"),
		},
	}

	peer1_2 := network.WireguardPeer{
		PublicKey:                   key1.PublicKey().String(),
		PersistentKeepaliveInterval: 10 * time.Second,
		AllowedIPs: []netaddr.IPPrefix{
			netaddr.MustParseIPPrefix("10.2.0.0/16"),
			netaddr.MustParseIPPrefix("10.2.0.0/24"),
		},
	}

	assert.True(t, peer1.Equal(&peer1))
	assert.False(t, peer1.Equal(&peer2))
	assert.False(t, peer1.Equal(&peer1_1))
	assert.True(t, peer1.Equal(&peer1_2))
}

func TestWireguardSpecZero(t *testing.T) {
	zeroSpec := network.WireguardSpec{}

	assert.True(t, zeroSpec.IsZero())
}

func TestWireguardSpecDecode(t *testing.T) {
	priv, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub1, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub2, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	var spec network.WireguardSpec

	// decode in spec mode
	spec.Decode(&wgtypes.Device{
		PrivateKey:   priv,
		ListenPort:   30000,
		FirewallMark: 1,
		Peers: []wgtypes.Peer{
			{
				PublicKey:    pub1.PublicKey(),
				PresharedKey: priv,
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("10.2.0.3"),
					Port: 20000,
				},
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("172.24.0.0"),
						Mask: net.IPv4Mask(255, 255, 0, 0),
					},
				},
			},
			{
				PublicKey: pub2.PublicKey(),
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("172.25.0.0"),
						Mask: net.IPv4Mask(255, 255, 255, 0),
					},
				},
			},
		},
	}, false)

	expected := network.WireguardSpec{
		PrivateKey:   priv.String(),
		ListenPort:   30000,
		FirewallMark: 1,
		Peers: []network.WireguardPeer{
			{
				PublicKey:    pub1.PublicKey().String(),
				PresharedKey: priv.String(),
				Endpoint:     "10.2.0.3:20000",
				AllowedIPs: []netaddr.IPPrefix{
					netaddr.MustParseIPPrefix("172.24.0.0/16"),
				},
			},
			{
				PublicKey: pub2.PublicKey().String(),
				AllowedIPs: []netaddr.IPPrefix{
					netaddr.MustParseIPPrefix("172.25.0.0/24"),
				},
			},
		},
	}

	assert.Equal(t, expected, spec)
	assert.True(t, expected.Equal(&spec))

	// zeroed out listen port is still acceptable on the right side
	spec.ListenPort = 0
	assert.True(t, expected.Equal(&spec))

	// ... but not on the left side
	expected.ListenPort = 0
	spec.ListenPort = 30000
	assert.False(t, expected.Equal(&spec))

	var zeroSpec network.WireguardSpec

	assert.False(t, zeroSpec.Equal(&spec))
}

func TestWireguardSpecDecodeStatus(t *testing.T) {
	priv, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	var spec network.WireguardSpec

	// decode in status mode
	spec.Decode(&wgtypes.Device{
		PrivateKey:   priv,
		PublicKey:    priv.PublicKey(),
		ListenPort:   30000,
		FirewallMark: 1,
	}, true)

	expected := network.WireguardSpec{
		PublicKey:    priv.PublicKey().String(),
		ListenPort:   30000,
		FirewallMark: 1,
		Peers:        []network.WireguardPeer{},
	}

	assert.Equal(t, expected, spec)
}

func TestWireguardSpecEncode(t *testing.T) {
	priv, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub1, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub2, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	// make sure pub1 < pub2
	if pub1.PublicKey().String() > pub2.PublicKey().String() {
		pub1, pub2 = pub2, pub1
	}

	specV1 := network.WireguardSpec{
		PrivateKey:   priv.String(),
		ListenPort:   30000,
		FirewallMark: 1,
		Peers: []network.WireguardPeer{
			{
				PublicKey: pub1.PublicKey().String(),
				Endpoint:  "10.2.0.3:20000",
				AllowedIPs: []netaddr.IPPrefix{
					netaddr.MustParseIPPrefix("172.24.0.0/16"),
				},
			},
			{
				PublicKey: pub2.PublicKey().String(),
				AllowedIPs: []netaddr.IPPrefix{
					netaddr.MustParseIPPrefix("172.25.0.0/24"),
				},
			},
		},
	}

	specV1.Sort()

	var zero network.WireguardSpec

	zero.Decode(&wgtypes.Device{}, false)
	zero.Sort()

	// from zero (empty) config to config with two peers
	delta, err := specV1.Encode(&zero)
	require.NoError(t, err)

	assert.Equal(t, &wgtypes.Config{
		PrivateKey:   &priv,
		ListenPort:   pointer.ToInt(30000),
		FirewallMark: pointer.ToInt(1),
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: pub1.PublicKey(),
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("10.2.0.3"),
					Port: 20000,
				},
				PersistentKeepaliveInterval: pointer.ToDuration(0),
				ReplaceAllowedIPs:           true,
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("172.24.0.0").To4(),
						Mask: net.IPv4Mask(255, 255, 0, 0),
					},
				},
			},
			{
				PublicKey:                   pub2.PublicKey(),
				PersistentKeepaliveInterval: pointer.ToDuration(0),
				ReplaceAllowedIPs:           true,
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("172.25.0.0").To4(),
						Mask: net.IPv4Mask(255, 255, 255, 0),
					},
				},
			},
		},
	}, delta)

	// noop
	delta, err = specV1.Encode(&specV1)
	require.NoError(t, err)

	assert.Equal(t, &wgtypes.Config{}, delta)

	// delete peer2
	specV2 := network.WireguardSpec{
		PrivateKey:   priv.String(),
		ListenPort:   30000,
		FirewallMark: 1,
		Peers: []network.WireguardPeer{
			{
				PublicKey: pub1.PublicKey().String(),
				Endpoint:  "10.2.0.3:20000",
				AllowedIPs: []netaddr.IPPrefix{
					netaddr.MustParseIPPrefix("172.24.0.0/16"),
				},
			},
		},
	}

	delta, err = specV2.Encode(&specV1)
	require.NoError(t, err)

	assert.Equal(t, &wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: pub2.PublicKey(),
				Remove:    true,
			},
		},
	}, delta)

	// update peer1, firewallMark
	specV3 := network.WireguardSpec{
		PrivateKey:   priv.String(),
		ListenPort:   30000,
		FirewallMark: 2,
		Peers: []network.WireguardPeer{
			{
				PublicKey:    pub1.PublicKey().String(),
				PresharedKey: priv.String(),
				AllowedIPs: []netaddr.IPPrefix{
					netaddr.MustParseIPPrefix("172.24.0.0/16"),
				},
			},
		},
	}

	delta, err = specV3.Encode(&specV2)
	require.NoError(t, err)

	assert.Equal(t, &wgtypes.Config{
		FirewallMark: pointer.ToInt(2),
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   pub1.PublicKey(),
				PresharedKey:                &priv,
				PersistentKeepaliveInterval: pointer.ToDuration(0),
				ReplaceAllowedIPs:           true,
				AllowedIPs: []net.IPNet{
					{
						IP:   net.ParseIP("172.24.0.0").To4(),
						Mask: net.IPv4Mask(255, 255, 0, 0),
					},
				},
			},
		},
	}, delta)
}

func TestWireguardSpecMerge(t *testing.T) {
	priv, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub1, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub2, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	for _, tt := range []struct {
		name  string
		spec  network.WireguardSpec
		other network.WireguardSpec

		expected network.WireguardSpec
	}{
		{
			name: "zero",
		},
		{
			name: "speczero",
			other: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub2.String(),
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},

			expected: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub2.String(),
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},
		},
		{
			name: "otherzero",
			spec: network.WireguardSpec{
				PrivateKey:   priv.String(),
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1.String(),
					},
				},
			},

			expected: network.WireguardSpec{
				PrivateKey:   priv.String(),
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1.String(),
					},
				},
			},
		},
		{
			name: "mixed",
			spec: network.WireguardSpec{
				PrivateKey:   priv.String(),
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1.String(),
					},
				},
			},
			other: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub2.String(),
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},

			expected: network.WireguardSpec{
				PrivateKey:   priv.String(),
				FirewallMark: 34,
				ListenPort:   456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1.String(),
					},
					{
						PublicKey: pub2.String(),
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},
		},
		{
			name: "peerconflict",
			spec: network.WireguardSpec{
				PrivateKey:   priv.String(),
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey:                   pub1.String(),
						PersistentKeepaliveInterval: time.Second,
					},
				},
			},
			other: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1.String(),
						Endpoint:  "127.0.0.1:466",
					},
					{
						PublicKey: pub2.String(),
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},

			expected: network.WireguardSpec{
				PrivateKey:   priv.String(),
				FirewallMark: 34,
				ListenPort:   456,
				Peers: []network.WireguardPeer{
					{
						PublicKey:                   pub1.String(),
						PersistentKeepaliveInterval: time.Second,
					},
					{
						PublicKey: pub2.String(),
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			spec := tt.spec
			spec.Merge(tt.other)

			assert.Equal(t, tt.expected, spec)
		})
	}
}
