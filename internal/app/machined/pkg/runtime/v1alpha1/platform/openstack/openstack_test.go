// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack_test

import (
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
	o := &openstack.OpenStack{}

	var metadata openstack.MetadataConfig

	require.NoError(t, json.Unmarshal(rawMetadata, &metadata))

	var n openstack.NetworkConfig

	require.NoError(t, json.Unmarshal(rawNetwork, &n))

	ctx := t.Context()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
	eth0.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xa4, 0xbf, 0x00, 0x10, 0x20, 0x30}
	require.NoError(t, st.Create(ctx, eth0))

	eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
	eth1.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xa4, 0xbf, 0x00, 0x10, 0x20, 0x31}
	require.NoError(t, st.Create(ctx, eth1))

	eth2 := network.NewLinkStatus(network.NamespaceName, "eth2")
	eth2.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0xa4, 0xbf, 0x00, 0x10, 0x20, 0x33}
	require.NoError(t, st.Create(ctx, eth2))

	// Bond slaves

	eth3 := network.NewLinkStatus(network.NamespaceName, "eth3")
	eth3.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x4c, 0xd9, 0x8f, 0xb3, 0x34, 0xf8}
	require.NoError(t, st.Create(ctx, eth3))

	eth4 := network.NewLinkStatus(network.NamespaceName, "eth4")
	eth4.TypedSpec().PermanentAddr = nethelpers.HardwareAddr{0x4c, 0xd9, 0x8f, 0xb3, 0x34, 0xf7}
	require.NoError(t, st.Create(ctx, eth4))

	networkConfig, needsReconcile, err := o.ParseMetadata(ctx, &n, []netip.Addr{netip.MustParseAddr("1.2.3.4")}, &metadata, st)
	require.NoError(t, err)
	assert.False(t, needsReconcile)

	marshaled, err := yaml.Marshal(networkConfig)
	require.NoError(t, err)

	assert.Equal(t, expectedNetworkConfig, string(marshaled))
}
