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

// staticIfaceContext builds a minimal context with a static ETH0 and an
// optional ONEGATE_ENDPOINT.
func staticIfaceContext(endpoint string) []byte {
	ctx := `ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_IP = "192.168.1.92"
ETH0_MASK = "255.255.255.0"
`

	if endpoint != "" {
		ctx += `ONEGATE_ENDPOINT = "` + endpoint + `"` + "\n"
	}

	return []byte(ctx)
}

func linkLocalRoute(ip, outLink string) network.RouteSpecSpec {
	return network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		Destination: netip.PrefixFrom(netip.MustParseAddr(ip), 32),
		OutLinkName: outLink,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeLink,
	}
}

func scopeLinkRoutes(routes []network.RouteSpecSpec) []network.RouteSpecSpec {
	var out []network.RouteSpecSpec

	for _, r := range routes {
		if r.Scope == nethelpers.ScopeLink {
			out = append(out, r)
		}
	}

	return out
}

func TestOnegateProxyRoute(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	tests := []struct {
		name      string
		endpoint  string
		wantRoute *network.RouteSpecSpec
	}{
		{
			name:     "link-local with port and path emits scope-link /32 route",
			endpoint: "http://169.254.16.9:5030/RPC2",
			wantRoute: func() *network.RouteSpecSpec {
				r := linkLocalRoute("169.254.16.9", "eth0")

				return &r
			}(),
		},
		{
			name:     "link-local without port emits route",
			endpoint: "http://169.254.16.9/RPC2",
			wantRoute: func() *network.RouteSpecSpec {
				r := linkLocalRoute("169.254.16.9", "eth0")

				return &r
			}(),
		},
		{
			name:      "non-link-local IP emits no route",
			endpoint:  "http://10.0.0.1:5030/RPC2",
			wantRoute: nil,
		},
		{
			name:      "absent ONEGATE_ENDPOINT emits no route",
			endpoint:  "",
			wantRoute: nil,
		},
		{
			name:      "IPv6 URL emits no route",
			endpoint:  "http://[::1]:5030/RPC2",
			wantRoute: nil,
		},
		{
			name:      "malformed endpoint emits no route without panic",
			endpoint:  "not-a-url",
			wantRoute: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := o.ParseMetadata(st, staticIfaceContext(tt.endpoint))
			require.NoError(t, err)

			routes := scopeLinkRoutes(cfg.Routes)

			if tt.wantRoute == nil {
				assert.Empty(t, routes)
			} else {
				require.Len(t, routes, 1)
				assert.Equal(t, *tt.wantRoute, routes[0])
			}
		})
	}
}

func TestOnegateRouteAttachedToFirstStaticInterface(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	// ETH0=dhcp, ETH1=static — route must be on eth1 (first static).
	ctx := []byte(`ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_METHOD = "dhcp"
ETH1_MAC = "02:00:c0:a8:01:5d"
ETH1_IP = "192.168.1.92"
ETH1_MASK = "255.255.255.0"
ONEGATE_ENDPOINT = "http://169.254.16.9:5030/RPC2"
`)

	cfg, err := o.ParseMetadata(st, ctx)
	require.NoError(t, err)

	routes := scopeLinkRoutes(cfg.Routes)
	require.Len(t, routes, 1)
	assert.Equal(t, "eth1", routes[0].OutLinkName)
}
