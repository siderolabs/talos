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

//go:embed testdata/bgpinstanceconfig.yaml
var expectedBGPInstanceConfigDocument []byte

func bgpInstanceTestConfig() *network.BGPInstanceConfigV1Alpha1 {
	cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
	cfg.BGPLocalASN = 65001
	cfg.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}
	cfg.BGPAdvertise = []string{"dummy0"}
	cfg.BGPMultipath = true
	cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{
		{
			NeighborAddressConfig: meta.Addr{Addr: netip.MustParseAddr("10.5.0.1")},
			NeighborPeerASN:       65000,
			NeighborPassive:       true,
			NeighborHoldTime:      9 * time.Second,
		},
		{
			NeighborLinkConfig: "enp0s1",
			NeighborPeerASN:    65000,
			NeighborHoldTime:   9 * time.Second,
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

	marshaled, err := encoder.NewEncoder(bgpInstanceTestConfig(), encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, expectedBGPInstanceConfigDocument, marshaled)
}

func TestBGPConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedBGPInstanceConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, bgpInstanceTestConfig(), docs[0])
}

func TestBGPConfigUnmarshalEmptyBFD(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: BGPInstanceConfig
name: fabric
localASN: 65001
neighbors:
  - link: enp0s1
    bfd: {}
`))
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	cfg, ok := docs[0].(*network.BGPInstanceConfigV1Alpha1)
	require.True(t, ok)
	require.Len(t, cfg.BGPNeighborConfigs, 1)
	assert.NotNil(t, cfg.BGPNeighborConfigs[0].NeighborBFDConfig)
}

func TestBGPInstanceConfigMerge(t *testing.T) {
	t.Parallel()

	left := network.NewBGPInstanceConfigV1Alpha1("fabric")
	left.BGPLocalASN = 65001

	right := network.NewBGPInstanceConfigV1Alpha1("fabric")
	right.BGPLocalASN = 65002

	require.NoError(t, merge.Merge(left, right))
	assert.Equal(t, uint32(65002), left.BGPLocalASN)
	assert.False(t, left.RouterID().IsValid())

	base := network.NewBGPInstanceConfigV1Alpha1("fabric")
	base.BGPLocalASN = 65001
	base.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("10.0.0.1")}

	patch := network.NewBGPInstanceConfigV1Alpha1("fabric")
	patch.BGPLocalASN = 65001

	require.NoError(t, merge.Merge(base, patch))
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), base.RouterID())
}

func TestBGPInstanceConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.BGPInstanceConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "missing name and localASN",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				return network.NewBGPInstanceConfigV1Alpha1("")
			},
			expectedError:    "name must be specified\nlocalASN must be specified",
			expectedWarnings: []string{"BGPInstanceConfig has no neighbors configured"},
		},
		{
			name: "no neighbors",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
				cfg.BGPLocalASN = 65001

				return cfg
			},
			expectedWarnings: []string{"BGPInstanceConfig has no neighbors configured"},
		},
		{
			name: "IPv6 router ID",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
				cfg.BGPLocalASN = 65001
				cfg.BGPRouterID = meta.Addr{Addr: netip.MustParseAddr("2001:db8::1")}

				return cfg
			},
			expectedError:    "routerID must be an IPv4 address",
			expectedWarnings: []string{"BGPInstanceConfig has no neighbors configured"},
		},
		{
			name: "neighbor with neither address nor link",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{{NeighborPeerASN: 65000}}

				return cfg
			},
			expectedError: "neighbor[0]: exactly one of address or link must be set",
		},
		{
			name: "duplicate neighbor",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{
					{NeighborLinkConfig: "enp0s1"},
					{NeighborLinkConfig: "enp0s1"},
				}

				return cfg
			},
			expectedError: "neighbor[1]: duplicate neighbor \"enp0s1\"",
		},
		{
			name: "negative inline timers",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{{
					NeighborLinkConfig: "enp0s1",
					NeighborHoldTime:   -time.Second,
					NeighborBFDConfig: &network.BGPBFDConfig{
						BFDTransmitInterval: -time.Millisecond,
						BFDReceiveInterval:  -2 * time.Millisecond,
					},
				}}

				return cfg
			},
			expectedError: "neighbor[0]: holdTime must not be negative\nneighbor[0]: bfd.transmitInterval must not be negative\nneighbor[0]: bfd.receiveInterval must not be negative",
		},
		{
			name: "BFD in VRF",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("workload")
				cfg.BGPVRF = "vrf-workload"
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{{
					NeighborAddressConfig: meta.Addr{Addr: netip.MustParseAddr("192.0.2.1")},
					NeighborBFDConfig:     &network.BGPBFDConfig{},
				}}

				return cfg
			},
			expectedError: "neighbor[0]: bfd is not supported for VRF-bound BGP instances",
		},
		{
			name: "empty BFD uses defaults",
			cfg: func() *network.BGPInstanceConfigV1Alpha1 {
				cfg := network.NewBGPInstanceConfigV1Alpha1("fabric")
				cfg.BGPLocalASN = 65001
				cfg.BGPNeighborConfigs = []network.BGPNeighborConfig{{
					NeighborLinkConfig: "enp0s1",
					NeighborBFDConfig:  &network.BGPBFDConfig{},
				}}

				return cfg
			},
		},
		{name: "valid numbered and unnumbered", cfg: bgpInstanceTestConfig},
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
