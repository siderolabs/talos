// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestVXLANSpecRoundTrip(t *testing.T) {
	t.Parallel()

	spec := network.VXLANSpec{
		ID:       100,
		Local:    netip.MustParseAddr("10.255.0.1"),
		Port:     4789,
		Learning: false,
	}

	b, err := networkadapter.VXLANSpec(&spec, new(uint32(3))).Encode()
	require.NoError(t, err)

	var (
		decodedSpec        network.VXLANSpec
		decodedParentIndex uint32
	)

	require.NoError(t, networkadapter.VXLANSpec(&decodedSpec, &decodedParentIndex).Decode(b))

	require.Equal(t, spec, decodedSpec)
	require.EqualValues(t, 3, decodedParentIndex)
}

func TestVXLANSpecRoundTripIPv6Group(t *testing.T) {
	t.Parallel()

	spec := network.VXLANSpec{
		ID:       200,
		Group:    netip.MustParseAddr("ff02::1"),
		Learning: true,
	}

	b, err := networkadapter.VXLANSpec(&spec, new(uint32(5))).Encode()
	require.NoError(t, err)

	var (
		decodedSpec        network.VXLANSpec
		decodedParentIndex uint32
	)

	require.NoError(t, networkadapter.VXLANSpec(&decodedSpec, &decodedParentIndex).Decode(b))

	// the port is not set in the spec, but it is always encoded, as the kernel would otherwise pick
	// its own default (8472)
	require.Equal(t, networkadapter.NormalizeVXLANSpec(spec), decodedSpec)
	require.EqualValues(t, networkadapter.DefaultVXLANPort, decodedSpec.Port)
	require.EqualValues(t, 5, decodedParentIndex)
}

func TestVXLANSpecRoundTripNormalized(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		spec network.VXLANSpec
	}{
		{
			name: "IPv4-mapped local",
			spec: network.VXLANSpec{
				ID:    100,
				Local: netip.MustParseAddr("::ffff:10.255.0.1"),
			},
		},
		{
			name: "local with a zone",
			spec: network.VXLANSpec{
				ID:    100,
				Local: netip.MustParseAddr("fe80::1%eth0"),
			},
		},
		{
			name: "unspecified local",
			spec: network.VXLANSpec{
				ID:    100,
				Local: netip.MustParseAddr("0.0.0.0"),
			},
		},
		{
			name: "unspecified group",
			spec: network.VXLANSpec{
				ID:    100,
				Group: netip.MustParseAddr("::"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			b, err := networkadapter.VXLANSpec(&test.spec, new(uint32(7))).Encode()
			require.NoError(t, err)

			var decodedSpec network.VXLANSpec

			require.NoError(t, networkadapter.VXLANSpec(&decodedSpec, nil).Decode(b))

			// whatever the kernel reports back has to match the normalized spec, otherwise the link
			// would be re-created on every reconcile
			require.Equal(t, networkadapter.NormalizeVXLANSpec(test.spec), decodedSpec)
		})
	}
}
