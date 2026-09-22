// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRoutSpecMarshalYAML(t *testing.T) {
	t.Parallel()

	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet6,
		Destination: netip.MustParsePrefix("192.168.3.4/25"),
		Source:      netip.MustParseAddr("1.1.1.1"),
		Gateway:     netip.MustParseAddr("2.2.2.2"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		Scope:       nethelpers.ScopeHost,
		Type:        nethelpers.TypeAnycast,
		Flags:       nethelpers.RouteFlags(nethelpers.RouteOffload | nethelpers.RouteCloned),
		Protocol:    nethelpers.ProtocolBoot,
		ConfigLayer: network.ConfigPlatform,
		MTU:         1400,
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t,
		`family: inet6
dst: 192.168.3.4/25
src: 1.1.1.1
gateway: 2.2.2.2
outLinkName: eth0
table: local
priority: 1024
scope: host
type: anycast
flags: cloned,offload
protocol: boot
layer: platform
mtu: 1400
`,
		string(marshaled))

	var spec2 network.RouteSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}

func TestRoutSpecNormalize(t *testing.T) {
	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("0.0.0.0/0"),
		Source:      netip.MustParseAddr("0.0.0.0"),
		Gateway:     netip.MustParseAddr("0.0.0.0"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		ConfigLayer: network.ConfigPlatform,
		MTU:         1400,
	}

	normalizedFamily := spec.Normalize()

	assert.Equal(t, netip.Prefix{}, spec.Destination)
	assert.Equal(t, netip.Addr{}, spec.Source)
	assert.Equal(t, netip.Addr{}, spec.Gateway)
	assert.Equal(t, nethelpers.FamilyInet4, normalizedFamily)
	assert.Equal(t, nethelpers.ScopeGlobal, spec.Scope)
}

func TestRoutSpecNormalizeV6(t *testing.T) {
	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("::/0"),
		OutLinkName: "eth0",
		Table:       nethelpers.TableLocal,
		Priority:    1024,
		ConfigLayer: network.ConfigPlatform,
		MTU:         1400,
	}

	normalizedFamily := spec.Normalize()

	assert.Equal(t, netip.Prefix{}, spec.Destination)
	assert.Equal(t, netip.Addr{}, spec.Source)
	assert.Equal(t, netip.Addr{}, spec.Gateway)
	assert.Equal(t, nethelpers.FamilyInet6, normalizedFamily)
	assert.Equal(t, nethelpers.ScopeGlobal, spec.Scope)
}

func TestNormalizeNextHops(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		nexthops []network.RouteNextHop
		expected []network.RouteNextHop
	}{
		{
			name: "nil",
		},
		{
			name: "sorted by gateway",
			nexthops: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("192.0.2.20")},
				{Gateway: netip.MustParseAddr("192.0.2.10")},
			},
			expected: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("192.0.2.10")},
				{Gateway: netip.MustParseAddr("192.0.2.20")},
			},
		},
		{
			// unnumbered peers share the same link-local gateway on different links
			name: "sorted by link for equal gateways",
			nexthops: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("fe80::1"), OutLinkName: "eth1"},
				{Gateway: netip.MustParseAddr("fe80::1"), OutLinkName: "eth0"},
			},
			expected: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("fe80::1"), OutLinkName: "eth0"},
				{Gateway: netip.MustParseAddr("fe80::1"), OutLinkName: "eth1"},
			},
		},
		{
			name: "duplicates dropped",
			nexthops: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("2001:db8::1")},
				{Gateway: netip.MustParseAddr("2001:db8::2")},
				{Gateway: netip.MustParseAddr("2001:db8::1")},
			},
			expected: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("2001:db8::1")},
				{Gateway: netip.MustParseAddr("2001:db8::2")},
			},
		},
		{
			name: "different weights are distinct",
			nexthops: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("192.0.2.10"), Weight: 2},
				{Gateway: netip.MustParseAddr("192.0.2.10"), Weight: 1},
			},
			expected: []network.RouteNextHop{
				{Gateway: netip.MustParseAddr("192.0.2.10"), Weight: 1},
				{Gateway: netip.MustParseAddr("192.0.2.10"), Weight: 2},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, network.NormalizeNextHops(test.nexthops))
		})
	}
}

func TestRoutSpecNormalizeNextHops(t *testing.T) {
	t.Parallel()

	spec := network.RouteSpecSpec{
		Family:      nethelpers.FamilyInet4,
		Destination: netip.MustParsePrefix("0.0.0.0/0"),
		NextHops: []network.RouteNextHop{
			{Gateway: netip.MustParseAddr("192.0.2.20")},
			{Gateway: netip.MustParseAddr("192.0.2.10")},
			{Gateway: netip.MustParseAddr("192.0.2.20")},
		},
	}

	spec.Normalize()

	assert.Equal(t, []network.RouteNextHop{
		{Gateway: netip.MustParseAddr("192.0.2.10")},
		{Gateway: netip.MustParseAddr("192.0.2.20")},
	}, spec.NextHops)
	assert.Equal(t, nethelpers.ScopeGlobal, spec.Scope)
}
