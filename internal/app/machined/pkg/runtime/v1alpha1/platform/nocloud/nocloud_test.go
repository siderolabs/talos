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

//go:embed testdata/in-v1.yaml
var rawNetworkConfigV1 []byte

//go:embed testdata/in-v1-pnap.yaml
var rawNetworkConfigV1Pnap []byte

//go:embed testdata/in-v2-nocloud.yaml
var rawNetworkConfigV2Nocloud []byte

//go:embed testdata/in-v2-cloud-init.yaml
var rawNetworkConfigV2CloudInit []byte

//go:embed testdata/in-v2-serverscom.yaml
var rawNetworkConfigV2Serverscom []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v1-pnap.yaml
var expectedNetworkConfigV1Pnap string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

//go:embed testdata/expected-v2-serverscom.yaml
var expectedNetworkConfigV2Serverscom string

func TestParseNetworkConfig(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		raw  []byte

		expected              string
		expectedNeedsRecocile bool
	}{
		{
			name:     "V1",
			raw:      rawNetworkConfigV1,
			expected: expectedNetworkConfigV1,
		},
		{
			name:     "V1-pnap",
			raw:      rawNetworkConfigV1Pnap,
			expected: expectedNetworkConfigV1Pnap,
		},
		{
			name:                  "V2-nocloud",
			raw:                   rawNetworkConfigV2Nocloud,
			expected:              expectedNetworkConfigV2,
			expectedNeedsRecocile: true,
		},
		{
			name:                  "V2-cloud-init",
			raw:                   rawNetworkConfigV2CloudInit,
			expected:              expectedNetworkConfigV2,
			expectedNeedsRecocile: true,
		},
		{
			name:     "V2-servers.com",
			raw:      rawNetworkConfigV2Serverscom,
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

//go:embed testdata/metadata-nocloud.yaml
var rawMetadataNocloud []byte

func TestMedatada(t *testing.T) {
	t.Parallel()

	var md nocloud.MetadataConfig

	err := yaml.Unmarshal(rawMetadataNocloud, &md)
	require.NoError(t, err)

	assert.Equal(t, nocloud.MetadataConfig{
		InstanceID:  "80d6927ecb30c1707b12f38ed1211535930ff16e",
		InternalDNS: "talos-worker-3",
	}, md)
}

func TestExtractURL(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		userdata []byte

		expectedURL   string
		expectedError string
	}{
		{
			name: "valid include userdata URL",
			userdata: []byte(`https://metadataserver/userdata
`),
			expectedURL: "https://metadataserver/userdata",
		},
		{
			name: "multiple URLs is invalid",
			userdata: []byte(`
https://metadataserver1/userdata
https://metadataserver2/userdata
https://metadataserver3/userdata
`),
			expectedError: "multiple #include URLs found",
		},
		{
			name: "invalid URL",
			userdata: []byte(`
:/invalidurl/userdata`),
			expectedError: "missing protocol scheme",
		},
		{
			name: "no URL found",
			userdata: []byte(`
`),
			expectedError: "no #include URL found in nocloud configuration",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			u, err := nocloud.ExtractIncludeURL(test.userdata)

			if test.expectedError != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedURL, u.String())
			}
		})
	}
}
