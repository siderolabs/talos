// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBridgeMasterSpec(t *testing.T) {
	spec := network.BridgeMasterSpec{
		STP: network.STPSpec{
			Enabled: true,
		},
		VLAN: network.BridgeVLANSpec{
			FilteringEnabled: true,
		},
	}

	b, err := networkadapter.BridgeMasterSpec(&spec).Encode()
	require.NoError(t, err)

	var decodedSpec network.BridgeMasterSpec

	require.NoError(t, networkadapter.BridgeMasterSpec(&decodedSpec).Decode(b))

	require.Equal(t, spec, decodedSpec)
}
