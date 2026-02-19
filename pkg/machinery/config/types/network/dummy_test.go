// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/dummylinkconfig.yaml
var expectedDummyLinkConfigDocument []byte

func TestDummyLinkConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewDummyLinkConfigV1Alpha1("dummy1")
	cfg.HardwareAddressConfig = nethelpers.HardwareAddr{0x2e, 0x3c, 0x4d, 0x5e, 0x6f, 0x70}
	cfg.LinkUp = new(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/32"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedDummyLinkConfigDocument, marshaled)
}

func TestDummyLinkConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedDummyLinkConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.DummyLinkConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.DummyLinkKind,
		},
		MetaName:              "dummy1",
		HardwareAddressConfig: nethelpers.HardwareAddr{0x2e, 0x3c, 0x4d, 0x5e, 0x6f, 0x70},
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: new(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("192.168.1.100/32"),
				},
			},
		},
	}, docs[0])
}
