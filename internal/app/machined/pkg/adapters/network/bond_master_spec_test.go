// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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

func TestBondMasterSpecDecodeClearsTargets(t *testing.T) {
	initial := network.BondMasterSpec{
		Mode:         nethelpers.BondModeActiveBackup,
		ARPIPTargets: []netip.Addr{netip.MustParseAddr("198.51.100.254")},
		NSIP6Targets: []netip.Addr{netip.MustParseAddr("fd00::1")},
	}

	initialEncoded, err := networkadapter.BondMasterSpec(&initial).Encode()
	require.NoError(t, err)

	cleared := network.BondMasterSpec{
		Mode: nethelpers.BondModeActiveBackup,
	}

	clearedEncoded, err := networkadapter.BondMasterSpec(&cleared).Encode()
	require.NoError(t, err)

	var decodedSpec network.BondMasterSpec

	require.NoError(t, networkadapter.BondMasterSpec(&decodedSpec).Decode(initialEncoded))
	require.Equal(t, initial.ARPIPTargets, decodedSpec.ARPIPTargets)
	require.Equal(t, initial.NSIP6Targets, decodedSpec.NSIP6Targets)

	require.NoError(t, networkadapter.BondMasterSpec(&decodedSpec).Decode(clearedEncoded))
	require.Empty(t, decodedSpec.ARPIPTargets)
	require.Empty(t, decodedSpec.NSIP6Targets)
}
