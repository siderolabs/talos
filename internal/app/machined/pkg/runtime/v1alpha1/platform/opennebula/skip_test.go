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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const skipContextBase = `ETH0_MAC = "02:00:c0:a8:01:5c"
ETH0_IP = "192.168.1.92"
ETH0_MASK = "255.255.255.0"
NAME = "test"
`

func skipContext(extra string) []byte {
	return []byte(skipContextBase + extra)
}

func TestParseMethodSkip(t *testing.T) {
	t.Parallel()

	o := &opennebula.OpenNebula{}
	st := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, tc := range []struct {
		name          string
		extra         string
		wantAddrs     int
		wantLinks     int
		wantRoutes    int
		wantOperators []network.OperatorSpecSpec
	}{
		{
			name:       "METHOD=skip omits interface entirely",
			extra:      `ETH0_METHOD = "skip"`,
			wantAddrs:  0,
			wantLinks:  0,
			wantRoutes: 0,
		},
		{
			name:  "METHOD=skip with IP6_METHOD=dhcp emits DHCPv6 operator only",
			extra: "ETH0_METHOD = \"skip\"\nETH0_IP6_METHOD = \"dhcp\"",
			wantOperators: []network.OperatorSpecSpec{
				{
					Operator:  network.OperatorDHCP6,
					LinkName:  "eth0",
					RequireUp: true,
					DHCP6: network.DHCP6OperatorSpec{
						RouteMetric:         1,
						SkipHostnameRequest: true,
					},
					ConfigLayer: network.ConfigPlatform,
				},
			},
		},
		{
			name:       "METHOD=skip with IP6_METHOD=disable omits interface entirely",
			extra:      "ETH0_METHOD = \"skip\"\nETH0_IP6_METHOD = \"disable\"",
			wantAddrs:  0,
			wantLinks:  0,
			wantRoutes: 0,
		},
		{
			name:       "METHOD=skip with IP6_METHOD=skip omits interface entirely",
			extra:      "ETH0_METHOD = \"skip\"\nETH0_IP6_METHOD = \"skip\"",
			wantAddrs:  0,
			wantLinks:  0,
			wantRoutes: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			networkConfig, err := o.ParseMetadata(st, skipContext(tc.extra))
			require.NoError(t, err)

			assert.Len(t, networkConfig.Addresses, tc.wantAddrs)
			assert.Len(t, networkConfig.Links, tc.wantLinks)
			assert.Len(t, networkConfig.Routes, tc.wantRoutes)
			assert.Equal(t, tc.wantOperators, networkConfig.Operators)
		})
	}
}
