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

const ipv6ContextBase = `ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_IP = "192.168.1.92"
ETH0_MASK = "255.255.255.0"
NAME = "test"
`

func ipv6Context(extra string) []byte {
	return []byte(ipv6ContextBase + extra)
}

func TestParseIPv6Static(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	defaultGWRoute := func(gw string, priority uint32) network.RouteSpecSpec {
		return network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     netip.MustParseAddr(gw),
			OutLinkName: "eth0",
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      nethelpers.FamilyInet6,
			Priority:    priority,
			Scope:       nethelpers.ScopeGlobal,
		}
	}

	for _, tc := range []struct {
		name       string
		extra      string
		wantAddrs  []netip.Prefix
		wantRoutes []network.RouteSpecSpec
	}{
		{
			name:      "static IPv6 address with explicit prefix length",
			extra:     "ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_PREFIX_LENGTH = \"48\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/48")},
		},
		{
			name:      "ETH*_IPV6 legacy alias used when ETH*_IP6 absent",
			extra:     "ETH0_IPV6 = \"2001:db8::1\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
		},
		{
			name:      "prefix length defaults to 64",
			extra:     "ETH0_IP6 = \"2001:db8::1\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
		},
		{
			name:      "explicit prefix length respected",
			extra:     "ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_PREFIX_LENGTH = \"56\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/56")},
		},
		{
			name:      "ULA address emitted as second AddressSpecSpec",
			extra:     "ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_ULA = \"fd00::1\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64"), netip.MustParsePrefix("fd00::1/64")},
		},
		{
			name:       "IPv6 gateway emits default route with metric 1",
			extra:      "ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_GATEWAY = \"2001:db8::fffe\"",
			wantAddrs:  []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
			wantRoutes: []network.RouteSpecSpec{defaultGWRoute("2001:db8::fffe", 1)},
		},
		{
			name:       "ETH*_GATEWAY6 legacy alias used when ETH*_IP6_GATEWAY absent",
			extra:      "ETH0_IP6 = \"2001:db8::1\"\nETH0_GATEWAY6 = \"2001:db8::fffe\"",
			wantAddrs:  []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
			wantRoutes: []network.RouteSpecSpec{defaultGWRoute("2001:db8::fffe", 1)},
		},
		{
			name:       "ETH*_IP6_METRIC overrides default metric of 1",
			extra:      "ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_GATEWAY = \"2001:db8::fffe\"\nETH0_IP6_METRIC = \"100\"",
			wantAddrs:  []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
			wantRoutes: []network.RouteSpecSpec{defaultGWRoute("2001:db8::fffe", 100)},
		},
		{
			name:  "no IPv6 variables — no IPv6 addresses or routes",
			extra: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			networkConfig, err := o.ParseMetadata(st, ipv6Context(tc.extra))
			require.NoError(t, err)

			var ip6Addrs []netip.Prefix

			for _, a := range networkConfig.Addresses {
				if a.Family == nethelpers.FamilyInet6 {
					ip6Addrs = append(ip6Addrs, a.Address)
				}
			}

			assert.Equal(t, tc.wantAddrs, ip6Addrs)

			var ip6Routes []network.RouteSpecSpec

			for _, r := range networkConfig.Routes {
				if r.Family == nethelpers.FamilyInet6 {
					ip6Routes = append(ip6Routes, r)
				}
			}

			assert.Equal(t, tc.wantRoutes, ip6Routes)
		})
	}
}

func TestParseIPv6Errors(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	t.Run("malformed IPv6 address returns descriptive error", func(t *testing.T) {
		t.Parallel()

		ctx := ipv6Context("ETH0_IP6 = \"notanip\"")

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0")
		require.ErrorContains(t, err, "IPv6")
	})

	t.Run("malformed IPv6 gateway returns descriptive error", func(t *testing.T) {
		t.Parallel()

		ctx := ipv6Context("ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_GATEWAY = \"notanip\"")

		_, err := o.ParseMetadata(st, ctx)
		require.ErrorContains(t, err, "ETH0")
		require.ErrorContains(t, err, "gateway")
	})
}
