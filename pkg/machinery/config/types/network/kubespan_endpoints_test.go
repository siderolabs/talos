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
)

//go:embed testdata/kubespanendpointsconfig.yaml
var expectedKubespanEndpointsConfigDocument []byte

func TestKubespanEndpointsConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewKubespanEndpointsV1Alpha1()
	cfg.ExtraAnnouncedEndpointsConfig = []netip.AddrPort{
		netip.MustParseAddrPort("3.4.5.6:123"),
		netip.MustParseAddrPort("10.11.12.13:456"),
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubespanEndpointsConfigDocument, marshaled)
}

func TestKubespanEndpointsConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubespanEndpointsConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.KubespanEndpointsConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.KubespanEndpointsKind,
		},
		ExtraAnnouncedEndpointsConfig: []netip.AddrPort{
			netip.MustParseAddrPort("3.4.5.6:123"),
			netip.MustParseAddrPort("10.11.12.13:456"),
		},
	}, docs[0])
}
