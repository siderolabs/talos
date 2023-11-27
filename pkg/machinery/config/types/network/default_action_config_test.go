// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/defaultactionconfig.yaml
var expectedDefaultActionConfigDocument []byte

func TestDefaultActionConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewDefaultActionConfigV1Alpha1()
	cfg.Ingress = nethelpers.DefaultActionBlock

	marshaled, err := encoder.NewEncoder(cfg).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedDefaultActionConfigDocument, marshaled)
}

func TestDefaultActionConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedDefaultActionConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.DefaultActionConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.DefaultActionConfig,
		},
		Ingress: nethelpers.DefaultActionBlock,
	}, docs[0])
}
