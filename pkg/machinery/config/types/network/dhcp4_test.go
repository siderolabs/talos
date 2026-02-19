// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
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

//go:embed testdata/dhcpv4config.yaml
var expectedDHCPv4ConfigDocument []byte

func TestDHCPv4ConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewDHCPv4ConfigV1Alpha1("enp0s3")
	cfg.ConfigRouteMetric = 512
	cfg.ConfigIgnoreHostname = new(true)
	cfg.ConfigClientIdentifier = new(nethelpers.ClientIdentifierDUID)
	cfg.ConfigDUIDRaw = nethelpers.HardwareAddr{0x00, 0x01, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedDHCPv4ConfigDocument, marshaled)
}

func TestDHCPv4ConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedDHCPv4ConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.DHCPv4ConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.DHCPv4Kind,
		},
		MetaName:               "enp0s3",
		ConfigRouteMetric:      512,
		ConfigIgnoreHostname:   new(true),
		ConfigClientIdentifier: new(nethelpers.ClientIdentifierDUID),
		ConfigDUIDRaw:          nethelpers.HardwareAddr{0x00, 0x01, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45},
	}, docs[0])
}

func TestDHCPv4ConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		cfg              func() *network.DHCPv4ConfigV1Alpha1
		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "valid config with duidRaw",
			cfg: func() *network.DHCPv4ConfigV1Alpha1 {
				c := network.NewDHCPv4ConfigV1Alpha1("enp0s3")
				c.ConfigClientIdentifier = new(nethelpers.ClientIdentifierDUID)
				c.ConfigDUIDRaw = nethelpers.HardwareAddr{0x00, 0x01, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45}

				return c
			},
		},
		{
			name: "invalid config missing duidRaw",
			cfg: func() *network.DHCPv4ConfigV1Alpha1 {
				c := network.NewDHCPv4ConfigV1Alpha1("enp0s3")
				c.ConfigClientIdentifier = new(nethelpers.ClientIdentifierDUID)

				return c
			},
			expectedError: "duidRaw must be set if clientIdentifier is 'duid'",
		},
		{
			name: "invalid config duidRaw set without duid client identifier",
			cfg: func() *network.DHCPv4ConfigV1Alpha1 {
				c := network.NewDHCPv4ConfigV1Alpha1("enp0s3")
				c.ConfigDUIDRaw = nethelpers.HardwareAddr{0x00, 0x01, 0x00, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45}

				return c
			},
			expectedError: "duidRaw can only be set if clientIdentifier is 'duid'",
		},
		{
			name: "empty",
			cfg: func() *network.DHCPv4ConfigV1Alpha1 {
				return network.NewDHCPv4ConfigV1Alpha1("")
			},

			expectedError: "name must be specified",
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
