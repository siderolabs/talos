// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud_test

import (
	"context"
	_ "embed"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/nocloud"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

//go:embed testdata/metadata-v1.yaml
var rawMetadataV1 []byte

//go:embed testdata/metadata-v2-nocloud.yaml
var rawMetadataV2Nocloud []byte

//go:embed testdata/metadata-v2-cloud-init.yaml
var rawMetadataV2CloudInit []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

func TestParseMetadata(t *testing.T) {
	for _, tt := range []struct {
		name     string
		raw      []byte
		expected string
	}{
		{
			name:     "V1",
			raw:      rawMetadataV1,
			expected: expectedNetworkConfigV1,
		},
		{
			name:     "V2-nocloud",
			raw:      rawMetadataV2Nocloud,
			expected: expectedNetworkConfigV2,
		},
		{
			name:     "V2-cloud-init",
			raw:      rawMetadataV2CloudInit,
			expected: expectedNetworkConfigV2,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := &nocloud.Nocloud{}

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			bond0 := network.NewLinkStatus(network.NamespaceName, "bond0")
			bond0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf7} // this link is not a physical one, so it should be ignored
			bond0.TypedSpec().Type = nethelpers.LinkEther
			bond0.TypedSpec().Kind = "bond"
			require.NoError(t, st.Create(context.TODO(), bond0))

			eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
			eth0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf7}
			eth0.TypedSpec().Type = nethelpers.LinkEther
			eth0.TypedSpec().Kind = ""
			require.NoError(t, st.Create(context.TODO(), eth0))

			eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
			eth1.TypedSpec().HardwareAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf9} // this link has a permanent address, so hardware addr should be ignored
			eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf8}
			eth1.TypedSpec().Type = nethelpers.LinkEther
			eth1.TypedSpec().Kind = ""
			require.NoError(t, st.Create(context.TODO(), eth1))

			eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
			eth2.TypedSpec().HardwareAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf9} // this link doesn't have a permanent address, but only a hardware address
			eth2.TypedSpec().Type = nethelpers.LinkEther
			eth2.TypedSpec().Kind = ""
			require.NoError(t, st.Create(context.TODO(), eth2))

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

			networkConfig, err := n.ParseMetadata(m, st, &mc)
			require.NoError(t, err)
			networkConfig2, err := n.ParseMetadata(m, st, &mc2)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)
			marshaled2, err := yaml.Marshal(networkConfig2)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(marshaled))
			assert.Equal(t, tt.expected, string(marshaled2))
		})
	}
}
