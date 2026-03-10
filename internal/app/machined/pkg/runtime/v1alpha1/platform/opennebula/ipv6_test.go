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

func TestParseIPv6(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	gw6Route := func(gw string, priority uint32) network.RouteSpecSpec {
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

	dhcp6Op := func(metric uint32) network.OperatorSpecSpec {
		return network.OperatorSpecSpec{
			Operator:  network.OperatorDHCP6,
			LinkName:  "eth0",
			RequireUp: true,
			DHCP6: network.DHCP6OperatorSpec{
				RouteMetric:         metric,
				SkipHostnameRequest: true,
			},
			ConfigLayer: network.ConfigPlatform,
		}
	}

	for _, tc := range []struct {
		name          string
		extra         string
		wantAddrs     []netip.Prefix
		wantRoutes    []network.RouteSpecSpec
		wantOperators []network.OperatorSpecSpec
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
			wantRoutes: []network.RouteSpecSpec{gw6Route("2001:db8::fffe", 1)},
		},
		{
			name:       "ETH*_GATEWAY6 legacy alias used when ETH*_IP6_GATEWAY absent",
			extra:      "ETH0_IP6 = \"2001:db8::1\"\nETH0_GATEWAY6 = \"2001:db8::fffe\"",
			wantAddrs:  []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
			wantRoutes: []network.RouteSpecSpec{gw6Route("2001:db8::fffe", 1)},
		},
		{
			name:       "ETH*_IP6_METRIC overrides default metric of 1",
			extra:      "ETH0_IP6 = \"2001:db8::1\"\nETH0_IP6_GATEWAY = \"2001:db8::fffe\"\nETH0_IP6_METRIC = \"100\"",
			wantAddrs:  []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
			wantRoutes: []network.RouteSpecSpec{gw6Route("2001:db8::fffe", 100)},
		},
		{
			name:  "no IPv6 variables — no IPv6 output",
			extra: "",
		},
		{
			name:          "IP6_METHOD=dhcp emits OperatorDHCP6 with default metric 1",
			extra:         "ETH0_IP6_METHOD = \"dhcp\"",
			wantOperators: []network.OperatorSpecSpec{dhcp6Op(1)},
		},
		{
			name:          "IP6_METHOD=dhcp with IP6_METRIC uses custom metric",
			extra:         "ETH0_IP6_METHOD = \"dhcp\"\nETH0_IP6_METRIC = \"200\"",
			wantOperators: []network.OperatorSpecSpec{dhcp6Op(200)},
		},
		{
			name:  "IP6_METHOD=auto emits nothing",
			extra: "ETH0_IP6_METHOD = \"auto\"\nETH0_IP6 = \"2001:db8::1\"",
		},
		{
			name:  "IP6_METHOD=disable emits nothing even if IP6 is set",
			extra: "ETH0_IP6_METHOD = \"disable\"\nETH0_IP6 = \"2001:db8::1\"",
		},
		{
			name:      "IP6_METHOD=static with IP6 set emits address",
			extra:     "ETH0_IP6_METHOD = \"static\"\nETH0_IP6 = \"2001:db8::1\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
		},
		{
			name:      "IP6_METHOD absent and IP6 set uses static path",
			extra:     "ETH0_IP6 = \"2001:db8::1\"",
			wantAddrs: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/64")},
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

			var ip6Operators []network.OperatorSpecSpec

			for _, op := range networkConfig.Operators {
				if op.Operator == network.OperatorDHCP6 {
					ip6Operators = append(ip6Operators, op)
				}
			}

			assert.Equal(t, tc.wantOperators, ip6Operators)
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
