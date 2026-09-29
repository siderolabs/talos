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

//go:embed testdata/vxlanconfig.yaml
var expectedVXLANConfigDocument []byte

func TestVXLANConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
	cfg.VXLANID = 100
	cfg.VXLANLocal = meta.Addr{Addr: netip.MustParseAddr("10.255.0.1")}
	cfg.VXLANParent = "vtep0"
	cfg.VXLANPort = new(uint16(4789))
	cfg.VXLANLearning = new(false)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedVXLANConfigDocument, marshaled)
}

func TestVXLANConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedVXLANConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.VXLANConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.VXLANKind,
		},
		MetaName:      "vxlan900",
		VXLANID:       100,
		VXLANLocal:    meta.Addr{Addr: netip.MustParseAddr("10.255.0.1")},
		VXLANParent:   "vtep0",
		VXLANPort:     new(uint16(4789)),
		VXLANLearning: new(false),
	}, docs[0])
}

func TestVXLANConfigValidate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		cfg     func() *network.VXLANConfigV1Alpha1
		wantErr string
	}{
		{
			name: "valid",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANID = 100
				cfg.VXLANLocal = meta.Addr{Addr: netip.MustParseAddr("10.255.0.1")}
				cfg.VXLANParent = "vtep0"

				return cfg
			},
		},
		{
			name: "missing name",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("")
				cfg.VXLANID = 100
				cfg.VXLANParent = "vtep0"

				return cfg
			},
			wantErr: "name must be specified",
		},
		{
			name: "missing id",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANParent = "vtep0"

				return cfg
			},
			wantErr: "id must be specified",
		},
		{
			name: "missing parent",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANID = 100

				return cfg
			},
			wantErr: "parent must be specified",
		},
		{
			name: "local and group both set",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANID = 100
				cfg.VXLANParent = "vtep0"
				cfg.VXLANLocal = meta.Addr{Addr: netip.MustParseAddr("10.255.0.1")}
				cfg.VXLANGroup = meta.Addr{Addr: netip.MustParseAddr("239.1.1.1")}

				return cfg
			},
			wantErr: "only one of local or group can be specified",
		},
		{
			name: "unspecified local",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANID = 100
				cfg.VXLANParent = "vtep0"
				cfg.VXLANLocal = meta.Addr{Addr: netip.MustParseAddr("0.0.0.0")}

				return cfg
			},
			wantErr: "local must not be an unspecified address",
		},
		{
			name: "unspecified group",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANID = 100
				cfg.VXLANParent = "vtep0"
				cfg.VXLANGroup = meta.Addr{Addr: netip.MustParseAddr("::")}

				return cfg
			},
			wantErr: "group must not be an unspecified address",
		},
		{
			name: "id out of 24-bit range",
			cfg: func() *network.VXLANConfigV1Alpha1 {
				cfg := network.NewVXLANConfigV1Alpha1("vxlan900")
				cfg.VXLANID = 16777216
				cfg.VXLANParent = "vtep0"

				return cfg
			},
			wantErr: "id must not exceed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.cfg().Validate(validationMode{})

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
