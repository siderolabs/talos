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
)

//go:embed testdata/networkwificonfig.yaml
var expectedNetworkWifiConfigDocument []byte

func TestNetworkWifiConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewWifiConfigV1Alpha1("wlan0")
	cfg.WifiCountryCode = "NL"
	cfg.WifiNetworks = []network.WifiNetworkConfig{
		{
			WifiSSID: "HomeNetwork",
			WifiPSK:  "topsecretpassphrase",
		},
		{
			WifiSSID:   "HiddenNetwork",
			WifiPSK:    "anothersecret",
			WifiHidden: true,
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedNetworkWifiConfigDocument, marshaled)
}

func TestNetworkWifiConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedNetworkWifiConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.WifiConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.WifiKind,
		},
		MetaName:        "wlan0",
		WifiCountryCode: "NL",
		WifiNetworks: []network.WifiNetworkConfig{
			{
				WifiSSID: "HomeNetwork",
				WifiPSK:  "topsecretpassphrase",
			},
			{
				WifiSSID:   "HiddenNetwork",
				WifiPSK:    "anothersecret",
				WifiHidden: true,
			},
		},
	}, docs[0])
}

func TestNetworkWifiConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func() *network.WifiConfigV1Alpha1

		expectedError string
	}{
		{
			name: "empty",

			cfg: func() *network.WifiConfigV1Alpha1 {
				return network.NewWifiConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nat least one network must be specified",
		},
		{
			name: "bad country and PSK",

			cfg: func() *network.WifiConfigV1Alpha1 {
				cfg := network.NewWifiConfigV1Alpha1("wlan0")
				cfg.WifiCountryCode = "Netherlands"
				cfg.WifiNetworks = []network.WifiNetworkConfig{
					{
						WifiSSID: "HomeNetwork",
						WifiPSK:  "short",
					},
				}

				return cfg
			},

			expectedError: "country code must be a two-letter ISO 3166-1 alpha2 code\nPSK passphrase must be between 8 and 63 characters long (network index 0)",
		},
		{
			name: "valid",

			cfg: func() *network.WifiConfigV1Alpha1 {
				cfg := network.NewWifiConfigV1Alpha1("wlan0")
				cfg.WifiNetworks = []network.WifiNetworkConfig{
					{
						WifiSSID: "HomeNetwork",
						WifiPSK:  "topsecretpassphrase",
					},
				}

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := test.cfg().Validate(validationMode{})

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}

func TestNetworkWifiConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := network.NewWifiConfigV1Alpha1("wlan0")
	cfg.WifiNetworks = []network.WifiNetworkConfig{
		{
			WifiSSID: "HomeNetwork",
			WifiPSK:  "topsecretpassphrase",
		},
	}

	cfg.Redact("REDACTED")

	assert.Equal(t, "REDACTED", cfg.WifiNetworks[0].WifiPSK)
}
