// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestWireguardPeer(t *testing.T) {
	key1 := "2t4fMmV1fBhI6RgoUzHp9BoWLT7oq0C/fOV17f7FqTI="
	key2 := "zHyf80qsjQ1EfiXkjxaLf9K9VZ6YRwcXx8GrpXQ6/yQ="

	peer1 := network.WireguardPeer{
		PublicKey:                   key1,
		Endpoint:                    "127.0.0.1:1000",
		PersistentKeepaliveInterval: 10 * time.Second,
		AllowedIPs: []netip.Prefix{
			netip.MustParsePrefix("10.2.0.0/16"),
			netip.MustParsePrefix("10.2.0.0/24"),
		},
	}

	peer2 := network.WireguardPeer{
		PublicKey: key2,
		Endpoint:  "127.0.0.1:2000",
		AllowedIPs: []netip.Prefix{
			netip.MustParsePrefix("10.2.0.0/15"),
			netip.MustParsePrefix("10.3.0.0/28"),
		},
	}

	peer1_1 := network.WireguardPeer{
		PublicKey:                   key1,
		Endpoint:                    "127.0.0.1:1000",
		PersistentKeepaliveInterval: 10 * time.Second,
		AllowedIPs: []netip.Prefix{
			netip.MustParsePrefix("10.2.0.0/15"),
			netip.MustParsePrefix("10.3.0.0/28"),
		},
	}

	peer1_2 := network.WireguardPeer{
		PublicKey:                   key1,
		PersistentKeepaliveInterval: 10 * time.Second,
		AllowedIPs: []netip.Prefix{
			netip.MustParsePrefix("10.2.0.0/16"),
			netip.MustParsePrefix("10.2.0.0/24"),
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

func TestWireguardSpecMerge(t *testing.T) {
	priv := "KIT4Pe7jFbCnH+ZMwsqsIbX2xiTdmemQU9w9sYItqXY="
	pub1 := "VHlgUWcakWcZyrtKI476PJSdoINTc1G5PYO1SEkr4FQ="
	pub2 := "EiBteTHU1Dk3w9CYJtHFaSgkuZBVBZLEa+Y07xu+xno="

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
						PublicKey: pub2,
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},

			expected: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub2,
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},
		},
		{
			name: "otherzero",
			spec: network.WireguardSpec{
				PrivateKey:   priv,
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1,
					},
				},
			},

			expected: network.WireguardSpec{
				PrivateKey:   priv,
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1,
					},
				},
			},
		},
		{
			name: "mixed",
			spec: network.WireguardSpec{
				PrivateKey:   priv,
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1,
					},
				},
			},
			other: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub2,
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},

			expected: network.WireguardSpec{
				PrivateKey:   priv,
				FirewallMark: 34,
				ListenPort:   456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1,
					},
					{
						PublicKey: pub2,
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},
		},
		{
			name: "peerconflict",
			spec: network.WireguardSpec{
				PrivateKey:   priv,
				FirewallMark: 34,
				Peers: []network.WireguardPeer{
					{
						PublicKey:                   pub1,
						PersistentKeepaliveInterval: time.Second,
					},
				},
			},
			other: network.WireguardSpec{
				ListenPort: 456,
				Peers: []network.WireguardPeer{
					{
						PublicKey: pub1,
						Endpoint:  "127.0.0.1:466",
					},
					{
						PublicKey: pub2,
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},

			expected: network.WireguardSpec{
				PrivateKey:   priv,
				FirewallMark: 34,
				ListenPort:   456,
				Peers: []network.WireguardPeer{
					{
						PublicKey:                   pub1,
						PersistentKeepaliveInterval: time.Second,
					},
					{
						PublicKey: pub2,
						Endpoint:  "127.0.0.1:3445",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.spec
			spec.Merge(tt.other)

			assert.Equal(t, tt.expected, spec)
		})
	}
}
