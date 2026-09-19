// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package scaleway_test

import (
	_ "embed"
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/scaleway"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/metadata-v1.json
var rawMetadataV1 []byte

//go:embed testdata/metadata-v2.json
var rawMetadataV2 []byte

//go:embed testdata/metadata-v3.json
var rawMetadataV3 []byte

//go:embed testdata/metadata-v4.json
var rawMetadataV4 []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

//go:embed testdata/expected-v3.yaml
var expectedNetworkConfigV3 string

//go:embed testdata/expected-v4.yaml
var expectedNetworkConfigV4 string

func TestParseMetadata(t *testing.T) {
	p := &scaleway.Scaleway{}

	for _, tt := range []struct {
		name     string
		raw      []byte
		expected string
	}{
		{
			name:     "V1",
			raw:      rawMetadataV1,
			expected: expectedNetworkConfigV1,
		},
		{
			name:     "V2",
			raw:      rawMetadataV2,
			expected: expectedNetworkConfigV2,
		},
		{
			name:     "V3",
			raw:      rawMetadataV3,
			expected: expectedNetworkConfigV3,
		},
		{
			name:     "V4",
			raw:      rawMetadataV4,
			expected: expectedNetworkConfigV4,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var metadata instance.Metadata

			require.NoError(t, json.Unmarshal(tt.raw, &metadata))

			networkConfig, err := p.ParseMetadata(&metadata)
			require.NoError(t, err)

			marshaled, err := yaml.Marshal(networkConfig)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, string(marshaled))
		})
	}
}

func TestParseMetadataMultipleIPv4Addresses(t *testing.T) {
	var metadata instance.Metadata

	require.NoError(t, json.Unmarshal(rawMetadataV2, &metadata))

	metadata.PublicIpsV4 = append(metadata.PublicIpsV4, instance.MetadataIP{
		Address: "192.0.2.10",
		Gateway: "192.0.2.1",
		Netmask: "32",
	})

	networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
	require.NoError(t, err)

	assert.Contains(t, networkConfig.ExternalIPs, netip.MustParseAddr("192.0.2.10"))
	assert.Condition(t, func() bool {
		for _, address := range networkConfig.Addresses {
			if address.Address == netip.MustParsePrefix("192.0.2.10/32") {
				return true
			}
		}

		return false
	})

	var defaultRoutes, gatewayRoutes int

	for _, route := range networkConfig.Routes {
		if route.Family != nethelpers.FamilyInet4 {
			continue
		}

		switch route.Destination {
		case netip.Prefix{}:
			defaultRoutes++

			assert.Equal(t, netip.MustParseAddr("11.22.222.1"), route.Gateway)
		case netip.MustParsePrefix("11.22.222.1/32"):
			gatewayRoutes++
		}
	}

	assert.Equal(t, 1, defaultRoutes)
	assert.Equal(t, 1, gatewayRoutes)
	require.Len(t, networkConfig.Operators, 1)
	assert.True(t, networkConfig.Operators[0].DHCP4.SkipRoutes)
}
