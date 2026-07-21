// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/vethconfig.yaml
var expectedVethConfigDocument []byte

func TestVethConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewVethConfigV1Alpha1("veth-metallb", "veth-router")
	cfg.LinkUp = new(true)
	cfg.LinkMTU = 1500
	cfg.LinkAddresses = []network.AddressConfig{{AddressAddress: netip.MustParsePrefix("fda1::1/127")}}
	cfg.VethPeer.LinkMTU = 1400
	cfg.VethPeer.LinkAddresses = []network.AddressConfig{{AddressAddress: netip.MustParsePrefix("fda1::/127")}}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedVethConfigDocument, marshaled)
}

func TestVethConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedVethConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.VethConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.VethKind,
		},
		MetaName: "veth-metallb",
		VethPeer: network.VethPeerConfig{
			VethPeerName: "veth-router",
			CommonLinkConfig: network.CommonLinkConfig{
				LinkMTU:       1400,
				LinkAddresses: []network.AddressConfig{{AddressAddress: netip.MustParsePrefix("fda1::/127")}},
			},
		},
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp:        new(true),
			LinkMTU:       1500,
			LinkAddresses: []network.AddressConfig{{AddressAddress: netip.MustParsePrefix("fda1::1/127")}},
		},
	}, docs[0])
}

func TestVethConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		linkName      string
		peerName      string
		expectedError string
	}{
		{name: "empty", expectedError: "name must be specified\npeer name must be specified"},
		{name: "empty peer", linkName: "veth0", expectedError: "peer name must be specified"},
		{name: "same name", linkName: "veth0", peerName: "veth0", expectedError: "name and peer name must be different"},
		{name: "long endpoint", linkName: strings.Repeat("a", 16), peerName: "veth1", expectedError: "name must not exceed 15 bytes"},
		{name: "invalid peer", linkName: "veth0", peerName: "veth/1", expectedError: "peer name must not contain '/' or ':'"},
		{name: "valid", linkName: "veth0", peerName: "veth1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := network.NewVethConfigV1Alpha1(test.linkName, test.peerName).Validate(validationMode{})
			assert.Empty(t, warnings)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}
