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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

//go:embed testdata/metadata-v1.json
var rawMetadataV1 []byte

//go:embed testdata/metadata-v2.json
var rawMetadataV2 []byte

//go:embed testdata/metadata-v3.json
var rawMetadataV3 []byte

//go:embed testdata/metadata-v4.json
var rawMetadataV4 []byte

// Sanitized networking metadata captured from DEV1-M instances in fr-par-1 on 2026-10-01.
// Identifiers, hostnames, addresses, and gateways are replaced; field shapes are preserved.
//
//go:embed testdata/metadata-routed-ipv4.json
var rawMetadataRoutedIPv4 []byte

//go:embed testdata/metadata-routed-ipv6.json
var rawMetadataRoutedIPv6 []byte

//go:embed testdata/expected-v1.yaml
var expectedNetworkConfigV1 string

//go:embed testdata/expected-v2.yaml
var expectedNetworkConfigV2 string

//go:embed testdata/expected-v3.yaml
var expectedNetworkConfigV3 string

//go:embed testdata/expected-v4.yaml
var expectedNetworkConfigV4 string

//go:embed testdata/expected-routed-ipv4.yaml
var expectedNetworkConfigRoutedIPv4 string

//go:embed testdata/expected-routed-ipv6.yaml
var expectedNetworkConfigRoutedIPv6 string

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
		{
			name:     "RoutedIPv4",
			raw:      rawMetadataRoutedIPv4,
			expected: expectedNetworkConfigRoutedIPv4,
		},
		{
			name:     "RoutedIPv6",
			raw:      rawMetadataRoutedIPv6,
			expected: expectedNetworkConfigRoutedIPv6,
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
	metadata.PublicIpsV6 = append(metadata.PublicIpsV6, instance.MetadataIP{
		Address: "2001:db8::10",
		Gateway: "fe80::1",
		Netmask: "64",
	})

	networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
	require.NoError(t, err)

	assert.Contains(t, networkConfig.ExternalIPs, netip.MustParseAddr("192.0.2.10"))

	for _, address := range networkConfig.Addresses {
		assert.False(t, address.Address.Addr().Is4(), "DHCP owns IPv4 addresses")
	}

	var defaultV6Routes int

	for _, route := range networkConfig.Routes {
		switch route.Family {
		case nethelpers.FamilyInet4:
			assert.Equal(t, netip.MustParsePrefix("169.254.42.42/32"), route.Destination, "only the metadata host route is platform-managed")
			assert.False(t, route.Gateway.IsValid(), "DHCP owns IPv4 gateways")
		case nethelpers.FamilyInet6:
			if route.Destination == (netip.Prefix{}) {
				defaultV6Routes++

				assert.Equal(t, netip.MustParseAddr("fe80::dc00:ff:fe12:3456"), route.Gateway)
			}
		}
	}

	assert.Equal(t, 1, defaultV6Routes)
	require.Len(t, networkConfig.Operators, 1)
	assert.False(t, networkConfig.Operators[0].DHCP4.SkipRoutes)
}

func TestParseMetadataLegacyIPv4Fallback(t *testing.T) {
	for _, tt := range []struct {
		name string
		ips  []instance.MetadataIP
	}{
		{name: "absent"},
		{name: "empty", ips: []instance.MetadataIP{}},
		{name: "malformed", ips: []instance.MetadataIP{{Address: "invalid"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var metadata instance.Metadata

			require.NoError(t, json.Unmarshal(rawMetadataV2, &metadata))

			metadata.PublicIpsV4 = tt.ips
			metadata.PublicIP.Address = "192.0.2.20"

			networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
			require.NoError(t, err)

			assert.Equal(t, []netip.Addr{
				netip.MustParseAddr("192.0.2.20"),
				netip.MustParseAddr("2001:111:222:3333::1"),
			}, networkConfig.ExternalIPs)
			require.Len(t, networkConfig.Operators, 1)
			assert.False(t, networkConfig.Operators[0].DHCP4.SkipRoutes)
		})
	}
}

func TestParseMetadataSuppressesDuplicateExternalIPs(t *testing.T) {
	var metadata instance.Metadata

	require.NoError(t, json.Unmarshal(rawMetadataV2, &metadata))

	metadata.PublicIpsV4 = append(metadata.PublicIpsV4, metadata.PublicIpsV4[0])
	metadata.PublicIpsV6 = append(metadata.PublicIpsV6, metadata.PublicIpsV6[0])

	networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
	require.NoError(t, err)

	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("11.22.222.222"),
		netip.MustParseAddr("2001:111:222:3333::1"),
	}, networkConfig.ExternalIPs)
}

func TestParseMetadataSkipsMalformedEntries(t *testing.T) {
	var metadata instance.Metadata

	require.NoError(t, json.Unmarshal(rawMetadataV2, &metadata))

	metadata.PublicIP.Address = ""
	metadata.PublicIpsV4 = []instance.MetadataIP{
		{Address: "invalid", Netmask: "32", Gateway: "192.0.2.1"},
		{Address: "192.0.2.2", Netmask: "invalid", Gateway: "192.0.2.1"},
		{Address: "192.0.2.3", Netmask: "32", Gateway: "192.0.2.1"},
		{Address: "192.0.2.4", Netmask: "32", Gateway: "192.0.2.1"},
	}
	metadata.PublicIpsV6 = []instance.MetadataIP{
		{Address: "invalid", Netmask: "64", Gateway: "fe80::1"},
		{Address: "2001:db8::2", Netmask: "invalid", Gateway: "fe80::1"},
		{Address: "2001:db8::4", Netmask: "64", Gateway: "fe80::1"},
	}

	networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
	require.NoError(t, err)

	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("192.0.2.2"),
		netip.MustParseAddr("192.0.2.3"),
		netip.MustParseAddr("192.0.2.4"),
		netip.MustParseAddr("2001:db8::4"),
	}, networkConfig.ExternalIPs)
	require.Len(t, networkConfig.Addresses, 1)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("2001:db8::4/64"),
	}, []netip.Prefix{
		networkConfig.Addresses[0].Address,
	})
}

func TestParseMetadataMalformedIPv4RetainsDHCP(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "modern"
		if legacy {
			name = "legacy"
		}

		t.Run(name, func(t *testing.T) {
			var metadata instance.Metadata

			require.NoError(t, json.Unmarshal(rawMetadataV2, &metadata))

			metadata.PublicIP.Address = "invalid"
			metadata.PublicIP.Family = ""
			metadata.PublicIpsV4 = []instance.MetadataIP{{Address: "invalid"}, {Address: "2001:db8::1"}}

			if legacy {
				metadata.PublicIpsV4 = nil
				metadata.PublicIP.Family = "inet"
			}

			networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
			require.NoError(t, err)

			require.Len(t, networkConfig.Operators, 1)
			assert.Equal(t, network.OperatorDHCP4, networkConfig.Operators[0].Operator)
			assert.False(t, networkConfig.Operators[0].DHCP4.SkipRoutes)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr("2001:111:222:3333::1")}, networkConfig.ExternalIPs)
			assert.NotEmpty(t, networkConfig.Addresses, "valid IPv6 configuration is preserved")
			assert.NotEmpty(t, networkConfig.Resolvers, "IPv6 DNS remains available without a usable advertised IPv4 address")
		})
	}
}

func TestParseMetadataLegacyIPv6Fallback(t *testing.T) {
	for _, tt := range []struct {
		name       string
		modernIPs  []instance.MetadataIP
		wantIP     string
		wantRoutes int
	}{
		{name: "absent", wantIP: "2001:111:222:3333::1", wantRoutes: 2},
		{
			name:       "malformed",
			modernIPs:  []instance.MetadataIP{{Address: "2001:db8::1", Netmask: "invalid", Gateway: "fe80::1"}},
			wantIP:     "2001:111:222:3333::1",
			wantRoutes: 2,
		},
		{
			name:       "usable without gateway suppresses legacy",
			modernIPs:  []instance.MetadataIP{{Address: "2001:db8::1", Netmask: "64"}},
			wantIP:     "2001:db8::1",
			wantRoutes: 1,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var metadata instance.Metadata

			require.NoError(t, json.Unmarshal(rawMetadataV1, &metadata))

			metadata.PublicIpsV6 = tt.modernIPs

			networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
			require.NoError(t, err)

			require.Len(t, networkConfig.Addresses, 1)
			assert.Equal(t, netip.MustParseAddr(tt.wantIP), networkConfig.Addresses[0].Address.Addr())
			assert.Equal(t, []netip.Addr{
				netip.MustParseAddr("11.22.222.222"),
				netip.MustParseAddr(tt.wantIP),
			}, networkConfig.ExternalIPs)
			assert.Len(t, networkConfig.Routes, tt.wantRoutes)
		})
	}
}

func TestParseMetadataIPv6AddressOrderAndFirstGateway(t *testing.T) {
	var metadata instance.Metadata

	require.NoError(t, json.Unmarshal(rawMetadataV3, &metadata))

	metadata.PublicIpsV6 = []instance.MetadataIP{
		{Address: "2001:db8::3", Netmask: "64"},
		{Address: "2001:db8::2", Netmask: "64", Gateway: "fe80::2"},
		{Address: "2001:db8::1", Netmask: "64", Gateway: "fe80::1"},
		{Address: "2001:db8::3", Netmask: "64", Gateway: "fe80::3"},
	}

	networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
	require.NoError(t, err)

	addresses := make([]netip.Prefix, 0, len(networkConfig.Addresses))

	for _, address := range networkConfig.Addresses {
		addresses = append(addresses, address.Address)
	}

	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("2001:db8::3/64"),
		netip.MustParsePrefix("2001:db8::2/64"),
		netip.MustParsePrefix("2001:db8::1/64"),
		netip.MustParsePrefix("2001:db8::3/64"),
	}, addresses)
	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("2001:db8::3"),
		netip.MustParseAddr("2001:db8::2"),
		netip.MustParseAddr("2001:db8::1"),
	}, networkConfig.ExternalIPs)
	require.Len(t, networkConfig.Routes, 2)
	assert.Equal(t, nethelpers.FamilyInet6, networkConfig.Routes[1].Family)
	assert.Equal(t, netip.Prefix{}, networkConfig.Routes[1].Destination)
	assert.Equal(t, netip.MustParseAddr("fe80::2"), networkConfig.Routes[1].Gateway)
}

func TestParseMetadataIPv4OnlyDoesNotAddIPv6DNS(t *testing.T) {
	var metadata instance.Metadata

	require.NoError(t, json.Unmarshal(rawMetadataV2, &metadata))

	metadata.PublicIpsV6 = nil
	metadata.IPv6.Address = ""

	networkConfig, err := (&scaleway.Scaleway{}).ParseMetadata(&metadata)
	require.NoError(t, err)
	assert.Empty(t, networkConfig.Resolvers)
	require.Len(t, networkConfig.Operators, 1)
	assert.Equal(t, network.OperatorDHCP4, networkConfig.Operators[0].Operator)
}
