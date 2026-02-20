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

func TestVRFMasterSpec(t *testing.T) {
	spec := network.VRFMasterSpec{
		Table: 4294967295,
	}

	b, err := networkadapter.VRFMasterSpec(&spec).Encode()
	require.NoError(t, err)

	var decodedSpec network.VRFMasterSpec

	require.NoError(t, networkadapter.VRFMasterSpec(&decodedSpec).Decode(b))

	require.Equal(t, spec, decodedSpec)
}
