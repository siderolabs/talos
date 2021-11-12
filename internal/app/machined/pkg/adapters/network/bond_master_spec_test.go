// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	networkadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/network"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/network"
)

func TestBondMasterSpec(t *testing.T) {
	spec := network.BondMasterSpec{
		Mode:      nethelpers.BondModeActiveBackup,
		MIIMon:    100,
		UpDelay:   200,
		DownDelay: 300,
	}

	b, err := networkadapter.BondMasterSpec(&spec).Encode()
	require.NoError(t, err)

	var decodedSpec network.BondMasterSpec

	require.NoError(t, networkadapter.BondMasterSpec(&decodedSpec).Decode(b))

	require.Equal(t, spec, decodedSpec)
}
