// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/openstack"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/network.json
var rawNetwork []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

func TestParseMetadata(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name                   string
		networkJSON            []byte
		metadataJSON           []byte
		extIPs                 []netip.Addr
		setupState             func(t *testing.T, ctx context.Context, st state.State)
		expectedNeedsReconcile bool
		expected               string
		checkResult            func(t *testing.T, cfg *runtime.PlatformNetworkConfig)
	}{
		{
			name:         "full config",
			networkJSON:  rawNetwork,
			metadataJSON: rawMetadata,
			extIPs:       []netip.Addr{netip.MustParseAddr("1.2.3.4")},
			setupState: func(t *testing.T, ctx context.Context, st state.State) {
				eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
				eth0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xa4, 0xbf, 0x00, 0x10, 0x20, 0x30}
				require.NoError(t, st.Create(ctx, eth0))

				eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
				eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xa4, 0xbf, 0x00, 0x10, 0x20, 0x31}
				require.NoError(t, st.Create(ctx, eth1))

				eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
				eth2.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xa4, 0xbf, 0x00, 0x10, 0x20, 0x33}
				require.NoError(t, st.Create(ctx, eth2))

				eth3 := network.NewLinkStatus(network.NamespaceName, "eth3")
				eth3.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x4c, 0xd9, 0x8f, 0xb3, 0x34, 0xf8}
				require.NoError(t, st.Create(ctx, eth3))

				eth4 := network.NewLinkStatus(network.NamespaceName, "eth4")
				eth4.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x4c, 0xd9, 0x8f, 0xb3, 0x34, 0xf7}
				require.NoError(t, st.Create(ctx, eth4))
			},
			expected: expectedNetworkConfig,
		},
		{
			name:        "HardwareAddr fallback",
			networkJSON: []byte(`{"links":[{"id":"iface1","type":"phy","ethernet_mac_address":"aa:bb:cc:dd:ee:ff","mtu":1500}],"networks":[{"id":"net1","link":"iface1","type":"ipv4_dhcp"}]}`),
			setupState: func(t *testing.T, ctx context.Context, st state.State) {
				eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
				eth0.TypedSpec().HardwareAddr = nethelpers.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
				require.NoError(t, st.Create(ctx, eth0))
			},
			checkResult: func(t *testing.T, cfg *runtime.PlatformNetworkConfig) {
				require.Len(t, cfg.Links, 1)
				assert.Equal(t, "eth0", cfg.Links[0].Name)
			},
		},
		{
			name:        "empty MAC does not match",
			networkJSON: []byte(`{"links":[{"id":"iface1","type":"phy","ethernet_mac_address":"","mtu":1500}],"networks":[{"id":"net1","link":"iface1","type":"ipv4_dhcp"}]}`),
			setupState: func(t *testing.T, ctx context.Context, st state.State) {
				eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
				require.NoError(t, st.Create(ctx, eth0))
			},
			expectedNeedsReconcile: true,
		},
		{
			name:        "MAC mismatch triggers reconcile",
			networkJSON: []byte(`{"links":[{"id":"iface1","type":"phy","ethernet_mac_address":"aa:bb:cc:dd:ee:ff","mtu":1500}],"networks":[{"id":"net1","link":"iface1","type":"ipv4_dhcp"}]}`),
			setupState: func(t *testing.T, ctx context.Context, st state.State) {
				eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
				eth0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
				require.NoError(t, st.Create(ctx, eth0))
			},
			expectedNeedsReconcile: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			tt.setupState(t, ctx, st)

			var (
				metadata openstack.MetadataConfig
				n        openstack.NetworkConfig
			)

			if tt.metadataJSON != nil {
				require.NoError(t, json.Unmarshal(tt.metadataJSON, &metadata))
			}

			require.NoError(t, json.Unmarshal(tt.networkJSON, &n))

			o := &openstack.OpenStack{}

			networkConfig, needsReconcile, err := o.ParseMetadata(ctx, &n, tt.extIPs, &metadata, st)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedNeedsReconcile, needsReconcile)

			if tt.expected != "" {
				marshaled, err := yaml.Marshal(networkConfig)
				require.NoError(t, err)

				assert.Equal(t, tt.expected, string(marshaled))
			}

			if tt.checkResult != nil {
				tt.checkResult(t, networkConfig)
			}
		})
	}
}
