// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/ethernetconfig.yaml
var expectedEthernetConfigDocument []byte

func TestEthernetConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewEthernetConfigV1Alpha1("enp0s1")
	cfg.RingsConfig = &network.EthernetRingsConfig{
		RX: pointer.To[uint32](16),
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEthernetConfigDocument, marshaled)
}

func TestEthernetConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedEthernetConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.EthernetConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.EthernetKind,
		},
		MetaName: "enp0s1",
		RingsConfig: &network.EthernetRingsConfig{
			RX: pointer.To[uint32](16),
		},
	}, docs[0])
}
