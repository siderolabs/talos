// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package equinixmetal_test

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/equinixmetal"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

//go:embed testdata/metadata.json
var rawMetadata []byte

//go:embed testdata/expected.yaml
var expectedNetworkConfig string

//go:embed testdata/metadata-2bonds.json
var rawMetadata2Bonds []byte

//go:embed testdata/expected-2bonds.yaml
var expectedNetworkConfig2Bonds string

func TestParseMetadata(t *testing.T) {
	p := &equinixmetal.EquinixMetal{}

	var m equinixmetal.MetadataConfig

	require.NoError(t, json.Unmarshal(rawMetadata, &m))

	ctx := context.Background()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
	eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf8}
	require.NoError(t, st.Create(ctx, eth1))

	eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
	eth2.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x68, 0x05, 0xca, 0xb8, 0xf1, 0xf9}
	require.NoError(t, st.Create(ctx, eth2))

	networkConfig, err := p.ParseMetadata(ctx, &m, st)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}

func TestParseMetadata2Bonds(t *testing.T) {
	p := &equinixmetal.EquinixMetal{}

	var m equinixmetal.MetadataConfig

	require.NoError(t, json.Unmarshal(rawMetadata2Bonds, &m))

	ctx := context.Background()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
	eth0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xe4, 0x43, 0x4b, 0xd0, 0x7b, 0x50}
	require.NoError(t, st.Create(ctx, eth0))

	eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
	eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xe4, 0x43, 0x4b, 0xd0, 0x7b, 0x51}
	require.NoError(t, st.Create(ctx, eth1))

	eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
	eth2.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xe4, 0x43, 0x4b, 0xd0, 0x7b, 0x52}
	require.NoError(t, st.Create(ctx, eth2))

	eth3 := network.NewLinkStatus(network.NamespaceName, "eth3")
	eth3.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xe4, 0x43, 0x4b, 0xd0, 0x7b, 0x53}
	require.NoError(t, st.Create(ctx, eth3))

	networkConfig, err := p.ParseMetadata(ctx, &m, st)
	require.NoError(t, err)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	fmt.Println(string(marshaled))

	assert.Equal(t, expectedNetworkConfig2Bonds, string(marshaled))
}
