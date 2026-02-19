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

//go:embed testdata/ethernetconfig.yaml
var expectedEthernetConfigDocument []byte

func TestEthernetConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewEthernetConfigV1Alpha1("enp0s1")
	cfg.RingsConfig = &network.EthernetRingsConfig{
		RX: new(uint32(16)),
	}
	cfg.FeaturesConfig = map[string]bool{
		"tx-checksum-ipv4": true,
	}
	cfg.ChannelsConfig = &network.EthernetChannelsConfig{
		Combined: new(uint32(1)),
	}
	cfg.WakeOnLANConfig = []nethelpers.WOLMode{
		nethelpers.WOLModeUnicast,
		nethelpers.WOLModeMulticast,
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
		FeaturesConfig: map[string]bool{
			"tx-checksum-ipv4": true,
		},
		RingsConfig: &network.EthernetRingsConfig{
			RX: new(uint32(16)),
		},
		ChannelsConfig: &network.EthernetChannelsConfig{
			Combined: new(uint32(1)),
		},
		WakeOnLANConfig: []nethelpers.WOLMode{
			nethelpers.WOLModeUnicast,
			nethelpers.WOLModeMulticast,
		},
	}, docs[0])
}

func TestEthernetValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.EthernetConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.EthernetConfigV1Alpha1 {
				return network.NewEthernetConfigV1Alpha1("")
			},

			expectedError: "name is required",
		},
		{
			name: "valid",
			cfg: func() *network.EthernetConfigV1Alpha1 {
				cfg := network.NewEthernetConfigV1Alpha1("enp0s1")
				cfg.FeaturesConfig = map[string]bool{
					"tx-checksum-ipv4": true,
				}
				cfg.RingsConfig = &network.EthernetRingsConfig{
					RX: new(uint32(16)),
				}

				return cfg
			},
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
