// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestMacVLANSpecRoundTrip(t *testing.T) {
	t.Parallel()

	for _, mode := range []nethelpers.MacvlanMode{
		nethelpers.MacvlanModePrivate,
		nethelpers.MacvlanModeVEPA,
		nethelpers.MacvlanModeBridge,
		nethelpers.MacvlanModePassthru,
		nethelpers.MacvlanModeSource,
	} {
		spec := network.MacVLANSpec{Mode: mode}

		b, err := networkadapter.MacVLANSpec(&spec).Encode()
		require.NoError(t, err)

		var decodedSpec network.MacVLANSpec

		require.NoError(t, networkadapter.MacVLANSpec(&decodedSpec).Decode(b))

		require.Equal(t, spec, decodedSpec)
	}
}
