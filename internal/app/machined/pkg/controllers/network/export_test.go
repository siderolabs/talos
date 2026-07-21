// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"iter"
	"net/netip"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Test exports for unexported route helpers (consumed by the external network_test package).
var (
	BuildMultipathForTest = buildMultipath
	MultipathEqualForTest = multipathEqual
)

// ResolveBGPRuntimeSpecForTest resolves a BGP spec against synthetic runtime status resources.
func ResolveBGPRuntimeSpecForTest(
	links []*network.LinkStatus,
	addresses []*network.AddressStatus,
	spec *network.BGPInstanceConfigSpec,
) (network.BGPInstanceConfigSpec, []netip.Prefix, error) {
	state := newBGPRuntimeState(
		func() iter.Seq[*network.LinkStatus] { return slices.Values(links) },
		func() iter.Seq[*network.AddressStatus] { return slices.Values(addresses) },
	)

	resolved, err := state.resolve(spec)
	if err != nil {
		return network.BGPInstanceConfigSpec{}, nil, err
	}

	return resolved, (&BGPController{}).advertisedPrefixes(&resolved, state), nil
}
