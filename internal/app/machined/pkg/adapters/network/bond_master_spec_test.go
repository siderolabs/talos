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
	t.Parallel()

	for _, test := range []struct {
		name string
		spec network.BondMasterSpec
	}{
		{
			name: "active-backup",
			spec: network.BondMasterSpec{
				Mode:      nethelpers.BondModeActiveBackup,
				MIIMon:    100,
				UpDelay:   200,
				DownDelay: 300,
			},
		},
		{
			name: "802.3ad no lacp",
			spec: network.BondMasterSpec{
				Mode:      nethelpers.BondMode8023AD,
				MIIMon:    100,
				UpDelay:   200,
				DownDelay: 300,
			},
		},
		{
			name: "802.3ad lacp on",
			spec: network.BondMasterSpec{
				Mode:         nethelpers.BondMode8023AD,
				MIIMon:       100,
				UpDelay:      200,
				DownDelay:    300,
				LACPRate:     nethelpers.LACPRateFast,
				ADLACPActive: new(nethelpers.ADLACPActiveOn),
			},
		},
		{
			name: "802.3ad lacp off",
			spec: network.BondMasterSpec{
				Mode:         nethelpers.BondMode8023AD,
				MIIMon:       100,
				UpDelay:      200,
				DownDelay:    300,
				ADLACPActive: new(nethelpers.ADLACPActiveOff),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			b, err := networkadapter.BondMasterSpec(&test.spec).Encode()
			require.NoError(t, err)

			var decodedSpec network.BondMasterSpec

			require.NoError(t, networkadapter.BondMasterSpec(&decodedSpec).Decode(b))

			require.Equal(t, test.spec, decodedSpec)
		})
	}
}

func TestBondMasterSpecDecodeClearsTargets(t *testing.T) {
	t.Parallel()

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
