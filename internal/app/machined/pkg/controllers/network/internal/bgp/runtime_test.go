// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"iter"
	"maps"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRuntimeStateResolve(t *testing.T) {
	t.Parallel()

	state := newRuntimeState(
		map[string]network.LinkStatusSpec{
			"vrf-blue": {Index: 10, Kind: network.LinkKindVRF},
			"dummy0":   {Index: 11, MasterIndex: 10, Alias: "node-ip"},
			"eth1":     {Index: 12, MasterIndex: 10, AltNames: []string{"fabric0"}},
			"eth0":     {Index: 13},
		},
		[]network.AddressStatusSpec{
			{Address: netip.MustParsePrefix("2001:db8::2/64"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("2001:db8::3/64"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("2001:db8:1::5/128"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("10.0.0.2/24"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("10.0.0.3/24"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("192.0.2.5/32"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("127.0.0.1/8"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("::1/128"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("fe80::1/64"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("ff02::1/16"), LinkName: "dummy0", LinkIndex: 11},
			{Address: netip.MustParsePrefix("198.51.100.1/24"), LinkName: "fabric0", LinkIndex: 12},
			{Address: netip.MustParsePrefix("172.16.0.2/24"), LinkName: "eth0", LinkIndex: 13},
		},
	)

	input := network.BGPInstanceConfigSpec{
		LocalASN:       65001,
		RouteSource:    netip.MustParseAddr("10.0.0.2"),
		AdvertiseLinks: []string{"node-ip"},
		VRF:            "vrf-blue",
		VRFTable:       nethelpers.RoutingTable(88),
		Neighbors: []network.BGPNeighborConfigSpec{
			{Link: "fabric0", PeerASN: 65002},
		},
	}

	resolved, err := state.Resolve(input)
	require.NoError(t, err)

	assert.Equal(t, "vrf-blue", resolved.Spec.VRF)
	// The VRF's kernel index travels with the resolved config: the listening socket is bound to
	// that device, so a VRF replaced under the same name must not reuse the running server.
	assert.EqualValues(t, 10, resolved.VRFIndex)
	assert.Equal(t, []string{"dummy0"}, resolved.Spec.AdvertiseLinks)
	assert.Equal(t, "eth1", resolved.Spec.Neighbors[0].Link)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.5/32"),
		netip.MustParsePrefix("2001:db8::/64"),
		netip.MustParsePrefix("2001:db8:1::5/128"),
	}, resolved.AdvertisedPrefixes)
	// a learned route must not replace an on-link route in the instance's VRF, other VRFs don't matter
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/24"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.5/32"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("2001:db8::/64"),
		netip.MustParsePrefix("2001:db8:1::5/128"),
		netip.MustParsePrefix("fe80::/64"),
		netip.MustParsePrefix("ff02::/16"),
	}, resolved.ConnectedPrefixes)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), resolved.RouterID)

	// Resolution clones mutable fields rather than modifying the input value.
	assert.Equal(t, []string{"node-ip"}, input.AdvertiseLinks)
	assert.Equal(t, "fabric0", input.Neighbors[0].Link)
}

func TestRuntimeStateValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		links map[string]network.LinkStatusSpec
		spec  network.BGPInstanceConfigSpec
		error string
	}{
		{
			name:  "missing route source",
			links: map[string]network.LinkStatusSpec{"dummy0": {Index: 1}},
			spec: network.BGPInstanceConfigSpec{
				RouterID:    netip.MustParseAddr("10.0.0.1"),
				RouteSource: netip.MustParseAddr("10.0.0.2"),
			},
			error: "route source: address 10.0.0.2 is not ready",
		},
		{
			name:  "VRF is not a VRF",
			links: map[string]network.LinkStatusSpec{"eth0": {Index: 1}},
			spec:  network.BGPInstanceConfigSpec{VRF: "eth0"},
			error: `link "eth0" is not a VRF`,
		},
		{
			name: "missing master",
			links: map[string]network.LinkStatusSpec{
				"eth0": {Index: 1, MasterIndex: 99},
			},
			spec:  network.BGPInstanceConfigSpec{AdvertiseLinks: []string{"eth0"}},
			error: "link master index 99 is not ready",
		},
		{
			name: "cyclic master chain",
			links: map[string]network.LinkStatusSpec{
				"eth0": {Index: 1, MasterIndex: 2},
				"eth1": {Index: 2, MasterIndex: 1},
			},
			spec:  network.BGPInstanceConfigSpec{AdvertiseLinks: []string{"eth0"}},
			error: `link "eth0" has a cyclic master chain`,
		},
		{
			name: "wrong routing domain",
			links: map[string]network.LinkStatusSpec{
				"vrf-blue": {Index: 10, Kind: network.LinkKindVRF},
				"eth0":     {Index: 1, MasterIndex: 10},
			},
			spec:  network.BGPInstanceConfigSpec{AdvertiseLinks: []string{"eth0"}},
			error: `link "eth0" belongs to VRF "vrf-blue", not the default routing domain`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := newRuntimeState(test.links, nil).Resolve(test.spec)
			require.ErrorContains(t, err, test.error)
		})
	}
}

func TestRuntimeStateRouterID(t *testing.T) {
	t.Parallel()

	state := newRuntimeState(
		map[string]network.LinkStatusSpec{"dummy0": {Index: 1}},
		[]network.AddressStatusSpec{
			{Address: netip.MustParsePrefix("10.0.0.9/24"), LinkName: "dummy0", LinkIndex: 1},
			{Address: netip.MustParsePrefix("2001:db8::2/64"), LinkName: "dummy0", LinkIndex: 1},
			{Address: netip.MustParsePrefix("10.0.0.2/32"), LinkName: "dummy0", LinkIndex: 1},
			{Address: netip.MustParsePrefix("10.0.0.2/24"), LinkName: "dummy0", LinkIndex: 1},
		},
	)

	auto, err := state.Resolve(network.BGPInstanceConfigSpec{AdvertiseLinks: []string{"dummy0"}})
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("10.0.0.2"), auto.RouterID)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.0.9")}, auto.RouterIDCandidates)

	configured, err := state.Resolve(network.BGPInstanceConfigSpec{
		RouterID:       netip.MustParseAddr("192.0.2.1"),
		AdvertiseLinks: []string{"dummy0"},
	})
	require.NoError(t, err)
	assert.Equal(t, netip.MustParseAddr("192.0.2.1"), configured.RouterID)

	missing, err := newRuntimeState(nil, nil).Resolve(network.BGPInstanceConfigSpec{})
	require.NoError(t, err)
	assert.False(t, missing.RouterID.IsValid())
}

func newRuntimeState(links map[string]network.LinkStatusSpec, addresses []network.AddressStatusSpec) *internalbgp.RuntimeState {
	linkStatuses := make([]*network.LinkStatus, 0, len(links))

	for _, name := range slices.Sorted(maps.Keys(links)) {
		status := network.NewLinkStatus(network.NamespaceName, name)
		*status.TypedSpec() = links[name]
		linkStatuses = append(linkStatuses, status)
	}

	addressStatuses := make([]*network.AddressStatus, 0, len(addresses))

	for _, spec := range addresses {
		status := network.NewAddressStatus(network.NamespaceName, spec.LinkName+"/"+spec.Address.String())
		*status.TypedSpec() = spec
		addressStatuses = append(addressStatuses, status)
	}

	return internalbgp.NewRuntimeState(func() iter.Seq[*network.LinkStatus] { return slices.Values(linkStatuses) }, slices.Values(addressStatuses))
}
