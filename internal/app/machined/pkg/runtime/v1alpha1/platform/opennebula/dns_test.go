// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package opennebula_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/opennebula"
)

func TestDNSMerge(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	mac := `ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_IP = "192.168.1.92"
ETH0_MASK = "255.255.255.0"
NAME = "test"
`

	for _, tc := range []struct {
		name           string
		extra          string
		wantDNS        []string
		wantSearch     []string
		wantNoResolver bool
	}{
		{
			name:           "global DNS only",
			extra:          `DNS = "9.9.9.9 1.1.1.1"`,
			wantDNS:        []string{"9.9.9.9", "1.1.1.1"},
			wantSearch:     nil,
			wantNoResolver: false,
		},
		{
			name:           "per-interface DNS only",
			extra:          `ETH0_DNS = "192.168.1.1 8.8.8.8"`,
			wantDNS:        []string{"192.168.1.1", "8.8.8.8"},
			wantSearch:     nil,
			wantNoResolver: false,
		},
		{
			name:           "global and per-interface DNS merged, global first",
			extra:          "DNS = \"9.9.9.9\"\nETH0_DNS = \"192.168.1.1\"",
			wantDNS:        []string{"9.9.9.9", "192.168.1.1"},
			wantSearch:     nil,
			wantNoResolver: false,
		},
		{
			name:           "global SEARCH_DOMAIN only",
			extra:          `SEARCH_DOMAIN = "global.example.com"`,
			wantDNS:        nil,
			wantSearch:     []string{"global.example.com"},
			wantNoResolver: false,
		},
		{
			name:           "per-interface search domain only",
			extra:          `ETH0_SEARCH_DOMAIN = "example.com"`,
			wantDNS:        nil,
			wantSearch:     []string{"example.com"},
			wantNoResolver: false,
		},
		{
			name:           "global and per-interface search domains merged, global first",
			extra:          "SEARCH_DOMAIN = \"global.example.com\"\nETH0_SEARCH_DOMAIN = \"example.com\"",
			wantDNS:        nil,
			wantSearch:     []string{"global.example.com", "example.com"},
			wantNoResolver: false,
		},
		{
			name:           "neither global nor per-interface set — no resolver emitted",
			extra:          "",
			wantNoResolver: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := []byte(mac + tc.extra)

			networkConfig, err := o.ParseMetadata(st, input)
			require.NoError(t, err)

			if tc.wantNoResolver {
				assert.Empty(t, networkConfig.Resolvers)

				return
			}

			require.Len(t, networkConfig.Resolvers, 1)

			resolver := networkConfig.Resolvers[0]

			var dnsStrs []string

			for _, ip := range resolver.DNSServers { //nolint:staticcheck
				dnsStrs = append(dnsStrs, ip.String())
			}

			assert.Equal(t, tc.wantDNS, dnsStrs)
			assert.Equal(t, tc.wantSearch, resolver.SearchDomains)
		})
	}
}

func TestDNSMergeError(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	base := `ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_IP = "192.168.1.92"
ETH0_MASK = "255.255.255.0"
NAME = "test"
`

	t.Run("malformed global DNS returns error", func(t *testing.T) {
		t.Parallel()

		_, err := o.ParseMetadata(st, []byte(base+`DNS = "notanip"`))
		require.ErrorContains(t, err, "failed to parse global DNS server")
		require.ErrorContains(t, err, "notanip")
	})

	t.Run("malformed per-interface DNS returns error with interface name", func(t *testing.T) {
		t.Parallel()

		_, err := o.ParseMetadata(st, []byte(base+`ETH0_DNS = "notanip"`))
		require.ErrorContains(t, err, "ETH0")
		require.ErrorContains(t, err, "notanip")
	})
}
