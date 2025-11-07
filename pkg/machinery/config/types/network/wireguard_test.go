// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"net/netip"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/wireguardconfig.yaml
var expectedWireguardConfigDocument []byte

//go:embed testdata/wireguardconfig_redacted.yaml
var expectedWireguardConfigRedactedDocument []byte

func TestWireguardConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewWireguardConfigV1Alpha1("wg.int")
	cfg.WireguardPrivateKey = "GA1E1VB+g41Dl0+UH2TMW9C5953y+moVg6JIIqkJbmw="
	cfg.WireguardListenPort = 5042
	cfg.WireguardFirewallMark = 0xb0
	cfg.WireguardPeers = []network.WireguardPeer{
		{
			WireguardPublicKey:  "735jkJdcVDninU5PzLJ/S+bfN6Q3QOk6svWrVLMJQAk=",
			WireguardAllowedIPs: []network.Prefix{{netip.MustParsePrefix("192.168.1.0/24")}},
		},
		{
			WireguardPublicKey:    "uvdlJNva1X8/OCOZM+0gGT4Yu9x20odd3AWbbQUF7nM=",
			WireguardPresharedKey: "6j4UMxwszrHVZZUjY8/SFsZMjgaHkxV7yp9Tz05btho=",
			WireguardEndpoint:     network.AddrPort{netip.MustParseAddrPort("10.3.4.3:2222")},
		},
	}
	cfg.LinkUp = pointer.To(true)
	cfg.LinkAddresses = []network.AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("192.168.1.100/32"),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedWireguardConfigDocument, marshaled)
}

func TestWireguardConfigRedact(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedWireguardConfigDocument)
	require.NoError(t, err)

	redactedProvider := provider.RedactSecrets("REDACTED")

	redacted, err := redactedProvider.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	t.Log(string(redacted))

	assert.Equal(t, expectedWireguardConfigRedactedDocument, redacted)
}

func TestWireguardConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedWireguardConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.WireguardConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.WireguardKind,
		},
		MetaName:              "wg.int",
		WireguardPrivateKey:   "GA1E1VB+g41Dl0+UH2TMW9C5953y+moVg6JIIqkJbmw=",
		WireguardListenPort:   5042,
		WireguardFirewallMark: 0xb0,
		WireguardPeers: []network.WireguardPeer{
			{
				WireguardPublicKey:  "735jkJdcVDninU5PzLJ/S+bfN6Q3QOk6svWrVLMJQAk=",
				WireguardAllowedIPs: []network.Prefix{{netip.MustParsePrefix("192.168.1.0/24")}},
			},
			{
				WireguardPublicKey:    "uvdlJNva1X8/OCOZM+0gGT4Yu9x20odd3AWbbQUF7nM=",
				WireguardPresharedKey: "6j4UMxwszrHVZZUjY8/SFsZMjgaHkxV7yp9Tz05btho=",
				WireguardEndpoint:     network.AddrPort{netip.MustParseAddrPort("10.3.4.3:2222")},
			},
		},
		CommonLinkConfig: network.CommonLinkConfig{
			LinkUp: pointer.To(true),
			LinkAddresses: []network.AddressConfig{
				{
					AddressAddress: netip.MustParsePrefix("192.168.1.100/32"),
				},
			},
		},
	}, docs[0])
}

func TestWireguardValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.WireguardConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *network.WireguardConfigV1Alpha1 {
				return network.NewWireguardConfigV1Alpha1("")
			},

			expectedError: "name must be specified\nwireguard private key must be specified",
		},
		{
			name: "invalid private key",
			cfg: func() *network.WireguardConfigV1Alpha1 {
				cfg := network.NewWireguardConfigV1Alpha1("wg.invalid")
				cfg.WireguardPrivateKey = "invalid-key"

				return cfg
			},

			expectedError: "wireguard private key is invalid: illegal base64 data at input byte 7",
		},
		{
			name: "invalid peer public key",
			cfg: func() *network.WireguardConfigV1Alpha1 {
				cfg := network.NewWireguardConfigV1Alpha1("wg.invalidpeer")
				cfg.WireguardPrivateKey = "0CWDz05gWWwVAwr4i5Zaz41k/FgIVYJmCsgAfLHUakU="
				cfg.WireguardPeers = []network.WireguardPeer{
					{
						WireguardPublicKey: "invalid-peer-key",
					},
				}

				return cfg
			},

			expectedError: "wireguard peer public key is invalid (peer index 0): illegal base64 data at input byte 7",
		},
		{
			name: "invalid peer preshared key",
			cfg: func() *network.WireguardConfigV1Alpha1 {
				cfg := network.NewWireguardConfigV1Alpha1("wg.invalidpreshared")
				cfg.WireguardPrivateKey = "OJ34O6J1z4ZZB+t16c+vYrzIrKddxyU3Z2eLhwYzqE8="
				cfg.WireguardPeers = []network.WireguardPeer{
					{
						WireguardPublicKey:    "fP+xJZvUA5n1Pi/f5wcPiV6tZ6fHwqcGaXe98NfEgkE=",
						WireguardPresharedKey: "invalid-preshared-key",
					},
				}

				return cfg
			},

			expectedError: "wireguard peer preshared key is invalid (peer index 0): illegal base64 data at input byte 7",
		},
		{
			name: "valid",
			cfg: func() *network.WireguardConfigV1Alpha1 {
				cfg := network.NewWireguardConfigV1Alpha1("wg.35")
				cfg.WireguardPrivateKey = "OJ34O6J1z4ZZB+t16c+vYrzIrKddxyU3Z2eLhwYzqE8="
				cfg.WireguardListenPort = 51820
				cfg.WireguardFirewallMark = 0x1
				cfg.WireguardPeers = []network.WireguardPeer{
					{
						WireguardPublicKey:  "735jkJdcVDninU5PzLJ/S+bfN6Q3QOk6svWrVLMJQAk=",
						WireguardAllowedIPs: []network.Prefix{{netip.MustParsePrefix("192.168.1.0/24")}},
					},
					{
						WireguardPublicKey:    "uvdlJNva1X8/OCOZM+0gGT4Yu9x20odd3AWbbQUF7nM=",
						WireguardPresharedKey: "6j4UMxwszrHVZZUjY8/SFsZMjgaHkxV7yp9Tz05btho=",
						WireguardEndpoint:     network.AddrPort{netip.MustParseAddrPort("10.3.4.3:2222")},
					},
				}
				cfg.LinkRoutes = []network.RouteConfig{
					{
						RouteDestination: network.Prefix{netip.MustParsePrefix("10.3.5.0/24")},
						RouteGateway:     network.Addr{netip.MustParseAddr("10.3.5.1")},
					},
					{
						RouteGateway: network.Addr{netip.MustParseAddr("fe80::1")},
					},
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
