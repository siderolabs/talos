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

//go:embed testdata/macvlanconfig.yaml
var expectedMacVLANConfigDocument []byte

func TestMacVLANConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewMacVLANConfigV1Alpha1("eth0.macvlan")
	cfg.MacVLANParent = "eth0"
	cfg.MacVLANMode = new(nethelpers.MacvlanModeBridge)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedMacVLANConfigDocument, marshaled)
}

func TestMacVLANConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedMacVLANConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.MacVLANConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.MacVLANKind,
		},
		MetaName:      "eth0.macvlan",
		MacVLANParent: "eth0",
		MacVLANMode:   new(nethelpers.MacvlanModeBridge),
	}, docs[0])
}

func TestMacVLANConfigValidate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		cfg     func() *network.MacVLANConfigV1Alpha1
		wantErr string
	}{
		{
			name: "valid",
			cfg: func() *network.MacVLANConfigV1Alpha1 {
				cfg := network.NewMacVLANConfigV1Alpha1("eth0.macvlan")
				cfg.MacVLANParent = "eth0"
				cfg.MacVLANMode = new(nethelpers.MacvlanModeBridge)

				return cfg
			},
		},
		{
			name: "valid without mode (defaults to bridge)",
			cfg: func() *network.MacVLANConfigV1Alpha1 {
				cfg := network.NewMacVLANConfigV1Alpha1("eth0.macvlan")
				cfg.MacVLANParent = "eth0"

				return cfg
			},
		},
		{
			name: "name missing",
			cfg: func() *network.MacVLANConfigV1Alpha1 {
				cfg := network.NewMacVLANConfigV1Alpha1("")
				cfg.MacVLANParent = "eth0"

				return cfg
			},
			wantErr: "name must be specified",
		},
		{
			name: "parent missing",
			cfg: func() *network.MacVLANConfigV1Alpha1 {
				cfg := network.NewMacVLANConfigV1Alpha1("eth0.macvlan")

				return cfg
			},
			wantErr: "parent must be specified",
		},
		{
			name: "invalid mode",
			cfg: func() *network.MacVLANConfigV1Alpha1 {
				cfg := network.NewMacVLANConfigV1Alpha1("eth0.macvlan")
				cfg.MacVLANParent = "eth0"

				mode := nethelpers.MacvlanMode(0xFF)
				cfg.MacVLANMode = &mode

				return cfg
			},
			wantErr: "invalid macvlan mode",
		},
		{
			name: "source mode unsupported",
			cfg: func() *network.MacVLANConfigV1Alpha1 {
				cfg := network.NewMacVLANConfigV1Alpha1("eth0.macvlan")
				cfg.MacVLANParent = "eth0"

				cfg.MacVLANMode = new(nethelpers.MacvlanModeSource)

				return cfg
			},
			wantErr: "source requires a list of MAC addresses",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := tt.cfg().Validate(validationMode{})
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Empty(t, warnings)
		})
	}
}
