// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestWireguardSpecDecode(t *testing.T) {
	if fipsmode.Strict() {
		t.Skip("skipping test in strict FIPS mode")
	}

	priv, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub1, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	pub2, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	var spec network.WireguardSpec

	// decode in spec mode
	networkadapter.WireguardSpec(&spec).Decode(&wgtypes.Device{
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
				AllowedIPs: []netip.Prefix{
					netip.MustParsePrefix("172.24.0.0/16"),
				},
			},
			{
				PublicKey: pub2.PublicKey().String(),
				AllowedIPs: []netip.Prefix{
					netip.MustParsePrefix("172.25.0.0/24"),
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
	if fipsmode.Strict() {
		t.Skip("skipping test in strict FIPS mode")
	}

	priv, err := wgtypes.GeneratePrivateKey()
	require.NoError(t, err)

	var spec network.WireguardSpec

	// decode in status mode
	networkadapter.WireguardSpec(&spec).Decode(&wgtypes.Device{
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
	if fipsmode.Strict() {
		t.Skip("skipping test in strict FIPS mode")
	}

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
				AllowedIPs: []netip.Prefix{
					netip.MustParsePrefix("172.24.0.0/16"),
				},
			},
			{
				PublicKey: pub2.PublicKey().String(),
				AllowedIPs: []netip.Prefix{
					netip.MustParsePrefix("172.25.0.0/24"),
				},
			},
		},
	}

	specV1.Sort()

	var zero network.WireguardSpec

	networkadapter.WireguardSpec(&zero).Decode(&wgtypes.Device{}, false)
	zero.Sort()

	// from zero (empty) config to config with two peers
	delta, err := networkadapter.WireguardSpec(&specV1).Encode(&zero)
	require.NoError(t, err)

	assert.Equal(t, &wgtypes.Config{
		PrivateKey:   &priv,
		ListenPort:   pointer.To(30000),
		FirewallMark: pointer.To(1),
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey: pub1.PublicKey(),
				Endpoint: &net.UDPAddr{
					IP:   net.ParseIP("10.2.0.3"),
					Port: 20000,
				},
				PersistentKeepaliveInterval: pointer.To[time.Duration](0),
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
				PersistentKeepaliveInterval: pointer.To[time.Duration](0),
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
	delta, err = networkadapter.WireguardSpec(&specV1).Encode(&specV1)
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
				AllowedIPs: []netip.Prefix{
					netip.MustParsePrefix("172.24.0.0/16"),
				},
			},
		},
	}

	delta, err = networkadapter.WireguardSpec(&specV2).Encode(&specV1)
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
				AllowedIPs: []netip.Prefix{
					netip.MustParsePrefix("172.24.0.0/16"),
				},
			},
		},
	}

	delta, err = networkadapter.WireguardSpec(&specV3).Encode(&specV2)
	require.NoError(t, err)

	assert.Equal(t, &wgtypes.Config{
		FirewallMark: pointer.To(2),
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   pub1.PublicKey(),
				PresharedKey:                &priv,
				PersistentKeepaliveInterval: pointer.To[time.Duration](0),
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
