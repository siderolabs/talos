// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/bgpconfig.yaml
var expectedBGPConfigDocument []byte

func bgpTestConfig() *network.BGPConfigV1Alpha1 {
	cfg := network.NewBGPConfigV1Alpha1()
	cfg.BGPLocalASN = 65001
	cfg.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	cfg.BGPAdvertise = []string{"dummy0"}
	cfg.BGPMultipath = true
	cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{
		{
			NeighborAddressConfig: meta.Addr{Addr: netip.MustParseAddr("10.5.0.1")},
			NeighborPeerASN:       65000,
		},
		{
			NeighborInterfaceConfig: "enp0s1",
			NeighborBFDConfig: &network.BGPBFDConfig{
				BFDTransmitInterval: 300 * time.Millisecond,
				BFDReceiveInterval:  300 * time.Millisecond,
				BFDDetectMultiplier: 3,
			},
		},
	}

	return cfg
}

func TestBGPConfigMarshalStability(t *testing.T) {
	t.Parallel()

	marshaled, err := encoder.NewEncoder(bgpTestConfig(), encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedBGPConfigDocument, marshaled)
}

func TestBGPConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedBGPConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, bgpTestConfig(), docs[0])
}

func TestBGPConfigMerge(t *testing.T) {
	t.Parallel()

	// regression: merging documents with an unset RouterID (meta.Addr) must not fail
	// trying to deep-merge netip's unexported fields.
	left := network.NewBGPConfigV1Alpha1()
	left.BGPLocalASN = 65001

	right := network.NewBGPConfigV1Alpha1()
	right.BGPLocalASN = 65002

	require.NoError(t, merge.Merge(left, right))
	assert.Equal(t, uint32(65002), left.BGPLocalASN)
	assert.False(t, left.RouterID().IsValid())

	// a non-zero RouterID in the patch overwrites; a zero one preserves the existing value.
	base := network.NewBGPConfigV1Alpha1()
	base.BGPLocalASN = 65001
	base.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}

	patch := network.NewBGPConfigV1Alpha1()
	patch.BGPLocalASN = 65001

	require.NoError(t, merge.Merge(base, patch))
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), base.RouterID())
}

func TestBGPConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.BGPConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name:             "missing localASN",
			cfg:              network.NewBGPConfigV1Alpha1,
			expectedError:    "localASN must be specified",
			expectedWarnings: []string{"BGPConfig has no neighbors configured"},
		},
		{
			name: "no neighbors",
			cfg: func() *network.BGPConfigV1Alpha1 {
				cfg := network.NewBGPConfigV1Alpha1()
				cfg.BGPLocalASN = 65001

				return cfg
			},
			expectedWarnings: []string{"BGPConfig has no neighbors configured"},
		},
		{
			name: "neighbor with neither address nor interface",
			cfg: func() *network.BGPConfigV1Alpha1 {
				cfg := network.NewBGPConfigV1Alpha1()
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{{NeighborPeerASN: 65000}}

				return cfg
			},
			expectedError: "neighbor[0]: exactly one of address or interface must be set",
		},
		{
			name: "neighbor with both address and interface",
			cfg: func() *network.BGPConfigV1Alpha1 {
				cfg := network.NewBGPConfigV1Alpha1()
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{
					{
						NeighborAddressConfig:   meta.Addr{Addr: netip.MustParseAddr("10.5.0.1")},
						NeighborInterfaceConfig: "enp0s1",
					},
				}

				return cfg
			},
			expectedError: "neighbor[0]: exactly one of address or interface must be set",
		},
		{
			name: "valid numbered + unnumbered",
			cfg:  bgpTestConfig,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
