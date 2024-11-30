// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/pcidriverrebindconfig.yaml
var expectedPCIDriverRebindConfigDocument []byte

func TestPCIDriverRebindConfigMarshal(t *testing.T) {
	t.Parallel()

	cfg := hardware.NewPCIDriverRebindConfigV1Alpha1()
	cfg.MetaName = "0000:04:00.00"
	cfg.PCITargetDriver = "vfio-pci"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, string(expectedPCIDriverRebindConfigDocument), string(marshaled))
}

func TestPCIDriverRebindConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedPCIDriverRebindConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &hardware.PCIDriverRebindConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       hardware.PCIDriverRebindConfig,
		},
		MetaName:        "0000:04:00.00",
		PCITargetDriver: "vfio-pci",
	}, docs[0])
}
