// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package opennebula_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/opennebula"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestParseRoutes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		routesStr string
		linkName  string
		expected  []network.RouteSpecSpec
		errMsg    string
	}{
		{
			name:      "empty string",
			routesStr: "",
			linkName:  "eth0",
			expected:  nil,
		},
		{
			name:      "whitespace only",
			routesStr: "   ,  ,  ",
			linkName:  "eth0",
			expected:  nil,
		},
		{
			name:      "legacy single route default metric",
			routesStr: "10.0.0.0 255.0.0.0 192.168.1.1",
			linkName:  "eth0",
			expected: []network.RouteSpecSpec{
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("10.0.0.0/8"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    network.DefaultRouteMetric,
					Scope:       nethelpers.ScopeGlobal,
				},
			},
		},
		{
			name:      "legacy single route custom metric",
			routesStr: "172.16.0.0 255.255.0.0 192.168.1.1 500",
			linkName:  "eth0",
			expected: []network.RouteSpecSpec{
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("172.16.0.0/16"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    500,
					Scope:       nethelpers.ScopeGlobal,
				},
			},
		},
		{
			name:      "cidr single route",
			routesStr: "10.0.0.0/8 via 192.168.1.1",
			linkName:  "eth0",
			expected: []network.RouteSpecSpec{
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("10.0.0.0/8"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    network.DefaultRouteMetric,
					Scope:       nethelpers.ScopeGlobal,
				},
			},
		},
		{
			name:      "cidr single route with metric",
			routesStr: "10.0.0.0/8 via 192.168.1.1 200",
			linkName:  "eth0",
			expected: []network.RouteSpecSpec{
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("10.0.0.0/8"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    200,
					Scope:       nethelpers.ScopeGlobal,
				},
			},
		},
		{
			name:      "multiple routes comma separated",
			routesStr: "10.0.0.0 255.0.0.0 192.168.1.1, 172.16.0.0 255.255.0.0 192.168.1.1 500",
			linkName:  "eth0",
			expected: []network.RouteSpecSpec{
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("10.0.0.0/8"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    network.DefaultRouteMetric,
					Scope:       nethelpers.ScopeGlobal,
				},
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("172.16.0.0/16"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    500,
					Scope:       nethelpers.ScopeGlobal,
				},
			},
		},
		{
			name:      "cidr host bits masked",
			routesStr: "10.1.2.0/8 via 192.168.1.1",
			linkName:  "eth0",
			expected: []network.RouteSpecSpec{
				{
					ConfigLayer: network.ConfigPlatform,
					Destination: netip.MustParsePrefix("10.0.0.0/8"),
					Gateway:     netip.MustParseAddr("192.168.1.1"),
					OutLinkName: "eth0",
					Table:       nethelpers.TableMain,
					Protocol:    nethelpers.ProtocolStatic,
					Type:        nethelpers.TypeUnicast,
					Family:      nethelpers.FamilyInet4,
					Priority:    network.DefaultRouteMetric,
					Scope:       nethelpers.ScopeGlobal,
				},
			},
		},
		{
			name:      "malformed gateway",
			routesStr: "10.0.0.0/8 via notanip",
			linkName:  "eth0",
			errMsg:    "failed to parse gateway",
		},
		{
			name:      "malformed cidr destination",
			routesStr: "notaprefix/8 via 192.168.1.1",
			linkName:  "eth0",
			errMsg:    "failed to parse destination",
		},
		{
			name:      "malformed legacy destination",
			routesStr: "notanip 255.0.0.0 192.168.1.1",
			linkName:  "eth0",
			errMsg:    "failed to parse destination",
		},
		{
			name:      "malformed metric",
			routesStr: "10.0.0.0/8 via 192.168.1.1 notanumber",
			linkName:  "eth0",
			errMsg:    "failed to parse metric",
		},
		{
			name:      "too few fields",
			routesStr: "10.0.0.0/8 via",
			linkName:  "eth0",
			errMsg:    "expected at least 3 fields",
		},
		{
			name:      "legacy too few fields",
			routesStr: "10.0.0.0 255.0.0.0",
			linkName:  "eth0",
			errMsg:    "expected at least 3 fields",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			routes, err := opennebula.ParseRoutes(tc.routesStr, tc.linkName)

			if tc.errMsg != "" {
				require.ErrorContains(t, err, tc.errMsg)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, routes)
		})
	}
}
