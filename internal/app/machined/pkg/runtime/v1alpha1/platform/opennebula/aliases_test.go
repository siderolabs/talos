// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package opennebula_test

import (
	"net/netip"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/opennebula"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const aliasContextBase = `ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_IP = "192.168.1.92"
ETH0_MASK = "255.255.255.0"
NAME = "test"
`

// aliasContext builds a minimal context string for alias testing.
func aliasContext(extra string) []byte {
	return []byte(aliasContextBase + extra)
}

func TestParseAliases(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	alias0IPv4 := network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("192.168.1.100/24"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		ConfigLayer: network.ConfigPlatform,
	}

	alias1IPv4 := network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("192.168.1.101/24"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		ConfigLayer: network.ConfigPlatform,
	}

	alias0IPv6 := network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("2001:db8::100/64"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet6,
		Scope:       nethelpers.ScopeGlobal,
		Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
		ConfigLayer: network.ConfigPlatform,
	}

	for _, tc := range []struct {
		name          string
		extra         string
		wantAliasAddr []network.AddressSpecSpec
	}{
		{
			name: "IPv4 alias included",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv4},
		},
		{
			name: "EXTERNAL=YES skips alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "YES"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: nil,
		},
		{
			name: "EXTERNAL=NO includes alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv4},
		},
		{
			name: "DETACH non-empty skips alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = "yes"`,
			wantAliasAddr: nil,
		},
		{
			name: "DETACH empty includes alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv4},
		},
		{
			name: "both DETACH non-empty and EXTERNAL=YES skips alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "YES"
ETH0_ALIAS0_DETACH = "yes"`,
			wantAliasAddr: nil,
		},
		{
			name: "multiple aliases sorted deterministically",
			extra: `ETH0_ALIAS1_MAC = "02:00:c0:a8:01:65"
ETH0_ALIAS1_IP = "192.168.1.101"
ETH0_ALIAS1_MASK = "255.255.255.0"
ETH0_ALIAS1_EXTERNAL = "NO"
ETH0_ALIAS1_DETACH = ""
ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			// ALIAS0 must appear before ALIAS1 regardless of map iteration order
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv4, alias1IPv4},
		},
		{
			name:          "no alias keys — no extra addresses",
			extra:         "",
			wantAliasAddr: nil,
		},
		{
			name: "IPv6 alias included",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv6},
		},
		{
			name: "ETH*_ALIAS*_IPV6 legacy alias used when IP6 absent",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IPV6 = "2001:db8::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv6},
		},
		{
			name: "IPv6 alias explicit prefix length respected",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_IP6_PREFIX_LENGTH = "48"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{
				{
					Address:     netip.MustParsePrefix("2001:db8::100/48"),
					LinkName:    "eth0",
					Family:      nethelpers.FamilyInet6,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					ConfigLayer: network.ConfigPlatform,
				},
			},
		},
		{
			name: "EXTERNAL=YES skips IPv6 alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_EXTERNAL = "YES"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: nil,
		},
		{
			name: "DETACH non-empty skips IPv6 alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = "yes"`,
			wantAliasAddr: nil,
		},
		{
			name: "mixed IPv4 and IPv6 aliases both emitted",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{alias0IPv4, alias0IPv6},
		},
		{
			name: "IPv6 ULA alias emits two IPv6 addresses",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_IP6_ULA = "fd00::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{
				alias0IPv6,
				{
					Address:     netip.MustParsePrefix("fd00::100/64"),
					LinkName:    "eth0",
					Family:      nethelpers.FamilyInet6,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					ConfigLayer: network.ConfigPlatform,
				},
			},
		},
		{
			name: "EXTERNAL=YES skips IPv6 ULA alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6_ULA = "fd00::100"
ETH0_ALIAS0_EXTERNAL = "YES"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: nil,
		},
		{
			name: "DETACH non-empty skips IPv6 ULA alias",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6_ULA = "fd00::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = "yes"`,
			wantAliasAddr: nil,
		},
		{
			name: "mixed IPv4 and IPv6 and ULA alias emits all three addresses",
			extra: `ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "192.168.1.100"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_IP6 = "2001:db8::100"
ETH0_ALIAS0_IP6_ULA = "fd00::100"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`,
			wantAliasAddr: []network.AddressSpecSpec{
				alias0IPv4,
				alias0IPv6,
				{
					Address:     netip.MustParsePrefix("fd00::100/64"),
					LinkName:    "eth0",
					Family:      nethelpers.FamilyInet6,
					Scope:       nethelpers.ScopeGlobal,
					Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
					ConfigLayer: network.ConfigPlatform,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			networkConfig, err := o.ParseMetadata(st, aliasContext(tc.extra))
			require.NoError(t, err)

			// The first address is always the primary ETH0 address; aliases follow.
			var aliasAddrs []network.AddressSpecSpec
			if len(networkConfig.Addresses) > 1 {
				aliasAddrs = networkConfig.Addresses[1:]
			}

			assert.Equal(t, tc.wantAliasAddr, aliasAddrs)
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	t.Run("malformed alias IPv4 returns descriptive error", func(t *testing.T) {
		t.Parallel()

		ctx := aliasContext(`ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP = "notanip"
ETH0_ALIAS0_MASK = "255.255.255.0"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`)

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0_ALIAS0")
		require.ErrorContains(t, err, "IPv4")
	})

	t.Run("malformed alias IPv6 returns descriptive error", func(t *testing.T) {
		t.Parallel()

		ctx := aliasContext(`ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6 = "notanip"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`)

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0_ALIAS0")
		require.ErrorContains(t, err, "IPv6")
	})

	t.Run("malformed alias IPv6 ULA returns error containing alias name and ULA", func(t *testing.T) {
		t.Parallel()

		ctx := aliasContext(`ETH0_ALIAS0_MAC = "02:00:c0:a8:01:64"
ETH0_ALIAS0_IP6_ULA = "notanip"
ETH0_ALIAS0_EXTERNAL = "NO"
ETH0_ALIAS0_DETACH = ""`)

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0_ALIAS0")
		require.ErrorContains(t, err, "ULA")
	})

	t.Run("malformed interface IPv6 address returns descriptive error", func(t *testing.T) {
		t.Parallel()

		ctx := aliasContext("ETH0_IP6 = \"notanip\"")

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0")
		require.ErrorContains(t, err, "IPv6")
	})

	t.Run("malformed IPv6 gateway returns descriptive error", func(t *testing.T) {
		t.Parallel()

		ctx := aliasContext("ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_GATEWAY = \"notanip\"")

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0")
		require.ErrorContains(t, err, "gateway")
	})
}
