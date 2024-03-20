// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vmware_test

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vmware"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

//go:embed testdata/metadata-match-by-mac.yaml
var rawMetadataMatchByMAC []byte

//go:embed testdata/expected-match-by-mac.yaml
var expectedNetworkConfigMatchByMAC string

//go:embed testdata/metadata-match-by-name.yaml
var rawMetadataMatchByName []byte

//go:embed testdata/expected-match-by-name.yaml
var expectedNetworkConfigMatchByName string

func TestApplyNetworkConfigV2a(t *testing.T) {
	for _, tt := range []struct {
		name     string
		raw      []byte
		expected string
	}{
		{
			name:     "byMAC",
			raw:      rawMetadataMatchByMAC,
			expected: expectedNetworkConfigMatchByMAC,
		},
		{
			name:     "byName",
			raw:      rawMetadataMatchByName,
			expected: expectedNetworkConfigMatchByName,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
			eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf8}
			require.NoError(t, st.Create(ctx, eth1))

			eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
			eth2.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf9}
			require.NoError(t, st.Create(ctx, eth2))

			var metadata vmware.NetworkConfig

			require.NoError(t, yaml.Unmarshal(tt.raw, &metadata))

			v := &vmware.VMware{}
			networkConfig := &runtime.PlatformNetworkConfig{}

			err := v.ApplyNetworkConfigV2(ctx, st, &metadata, networkConfig)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)

			fmt.Print(string(marshaled))

			assert.Equal(t, tt.expected, string(marshaled))
		})
	}
}
