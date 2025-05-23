// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud_test

import (
	"context"
	_ "embed"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/nocloud"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

//go:embed testdata/metadata-v1.yaml
var rawMetadataV1 []byte

//go:embed testdata/metadata-v1-pnap.yaml
var rawMetadataV1Pnap []byte

//go:embed testdata/metadata-v2-nocloud.yaml
var rawMetadataV2Nocloud []byte

//go:embed testdata/metadata-v2-cloud-init.yaml
var rawMetadataV2CloudInit []byte

//go:embed testdata/metadata-v2-serverscom.yaml
var rawMetadataV2Serverscom []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v1-pnap.yaml
var expectedNetworkConfigV1Pnap string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

//go:embed testdata/expected-v2-serverscom.yaml
var expectedNetworkConfigV2Serverscom string

func TestParseMetadata(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		raw  []byte

		expected              string
		expectedNeedsRecocile bool
	}{
		{
			name:     "V1",
			raw:      rawMetadataV1,
			expected: expectedNetworkConfigV1,
		},
		{
			name:     "V1-pnap",
			raw:      rawMetadataV1Pnap,
			expected: expectedNetworkConfigV1Pnap,
		},
		{
			name:                  "V2-nocloud",
			raw:                   rawMetadataV2Nocloud,
			expected:              expectedNetworkConfigV2,
			expectedNeedsRecocile: true,
		},
		{
			name:                  "V2-cloud-init",
			raw:                   rawMetadataV2CloudInit,
			expected:              expectedNetworkConfigV2,
			expectedNeedsRecocile: true,
		},
		{
			name:     "V2-servers.com",
			raw:      rawMetadataV2Serverscom,
			expected: expectedNetworkConfigV2Serverscom,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)

			n := &nocloud.Nocloud{}

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			devicesReady := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
			devicesReady.TypedSpec().Ready = true
			require.NoError(t, st.Create(ctx, devicesReady))

			bond0 := network.NewLinkStatus(network.NamespaceName, "bond0")
			bond0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf7} // this link is not a physical one, so it should be ignored
			bond0.TypedSpec().Type = nethelpers.LinkEther
			bond0.TypedSpec().Kind = "bond"
			require.NoError(t, st.Create(ctx, bond0))

			eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
			eth0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf7}
			eth0.TypedSpec().Type = nethelpers.LinkEther
			eth0.TypedSpec().Kind = ""
			require.NoError(t, st.Create(ctx, eth0))

			eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
			eth1.TypedSpec().HardwareAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf9} // this link has a permanent address, so hardware addr should be ignored
			eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf8}
			eth1.TypedSpec().Type = nethelpers.LinkEther
			eth1.TypedSpec().Kind = ""
			require.NoError(t, st.Create(ctx, eth1))

			eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
			eth2.TypedSpec().HardwareAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf9} // this link doesn't have a permanent address, but only a hardware address
			eth2.TypedSpec().Type = nethelpers.LinkEther
			eth2.TypedSpec().Kind = ""
			require.NoError(t, st.Create(ctx, eth2))

			eno1np0 := network.NewLinkStatus(network.NamespaceName, "eno1np0")
			eno1np0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr(must.Value(net.ParseMAC("3c:ec:ef:e0:45:28"))(t))
			eno1np0.TypedSpec().Type = nethelpers.LinkEther
			eno1np0.TypedSpec().Kind = ""
			require.NoError(t, st.Create(ctx, eno1np0))

			eno2np1 := network.NewLinkStatus(network.NamespaceName, "eno2np1")
			eno2np1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr(must.Value(net.ParseMAC("3c:ec:ef:e0:45:29"))(t))
			eno2np1.TypedSpec().Type = nethelpers.LinkEther
			eno2np1.TypedSpec().Kind = ""
			require.NoError(t, st.Create(ctx, eno2np1))

			m, err := nocloud.DecodeNetworkConfig(tt.raw)
			require.NoError(t, err)

			mc := nocloud.MetadataConfig{
				Hostname:    "talos.fqdn",
				InternalDNS: "talos.fqdn",
				InstanceID:  "0",
			}
			mc2 := nocloud.MetadataConfig{
				InternalDNS: "talos.fqdn",
				InstanceID:  "0",
			}

			networkConfig, needsReconcile, err := n.ParseMetadata(ctx, m, st, &mc)
			require.NoError(t, err)
			networkConfig2, needsReconcile2, err := n.ParseMetadata(ctx, m, st, &mc2)
			require.NoError(t, err)

			assert.Equal(t, needsReconcile, needsReconcile2)
			assert.Equal(t, tt.expectedNeedsRecocile, needsReconcile)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)
			marshaled2, err := yaml.Marshal(networkConfig2)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(marshaled))
			assert.Equal(t, tt.expected, string(marshaled2))
		})
	}
}

func TestExtractURL(t *testing.T) {
	u, err := nocloud.ExtractURL([]byte(`#include
https://metadataserver/userdata`))
	assert.NoError(t, err)
	assert.Equal(t, u.String(), "https://metadataserver/userdata")
}

func TestEventConfig(t *testing.T) {
	type test struct {
		name        string
		userdata    []byte
		expectedURL string
		errExpected bool
	}
	tests := []test{
		{
			name: "valid include userdata URL",
			userdata: []byte(`#include
			https://metadataserver/userdata
`),
			expectedURL: "https://metadataserver/userdata",
			errExpected: false,
		},
		{
			name: "multiple URL fetches first URL and ignore the rest",
			userdata: []byte(`#include
https://metadataserver1/userdata
https://metadataserver2/userdata
https://metadataserver3/userdata
`),
			expectedURL: "https://metadataserver1/userdata",
			errExpected: false,
		},
		{
			name: "invalid URL",
			userdata: []byte(`#include
:/invalidurl/userdata`),
			expectedURL: "",
			errExpected: true,
		},
		{
			name:        "no URL found",
			userdata:    []byte(`#include`),
			expectedURL: "",
			errExpected: true,
		},
		{
			name: "invalid URL",
			userdata: []byte(`#include
:/invalidurl/userdata`),
			expectedURL: "",
			errExpected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := nocloud.ExtractURL(tc.userdata)
			if tc.errExpected {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, u.String(), tc.expectedURL)
			}
		},
		)
	}
}
