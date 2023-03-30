// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/dashboard"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestEmptyFormData(t *testing.T) {
	formData := dashboard.NetworkConfigFormData{}

	config, err := formData.ToPlatformNetworkConfig()
	assert.NoError(t, err)

	assert.Equal(t, runtime.PlatformNetworkConfig{}, *config)
}

func TestBaseDataZeroOut(t *testing.T) {
	base := runtime.PlatformNetworkConfig{
		Addresses: []network.AddressSpecSpec{
			{
				LinkName: "foobar",
			},
		},
		Links: []network.LinkSpecSpec{
			{
				Name: "foobar",
			},
		},
		Routes: []network.RouteSpecSpec{
			{
				OutLinkName: "foobar",
			},
		},
		Hostnames: []network.HostnameSpecSpec{
			{
				Hostname: "foobar",
			},
		},
		Resolvers: []network.ResolverSpecSpec{
			{
				DNSServers: []netip.Addr{
					netip.MustParseAddr("1.2.3.4"),
				},
			},
		},
		TimeServers: []network.TimeServerSpecSpec{
			{
				NTPServers: []string{"foobar"},
			},
		},
		Operators: []network.OperatorSpecSpec{
			{
				LinkName: "foobar",
			},
		},
		ExternalIPs: []netip.Addr{
			netip.MustParseAddr("2.3.4.5"),
		},
		Metadata: &runtimeres.PlatformMetadataSpec{
			Platform: "foobar",
			Spot:     true,
		},
	}

	formData := dashboard.NetworkConfigFormData{
		Base: base,
	}

	config, err := formData.ToPlatformNetworkConfig()
	assert.NoError(t, err)

	// assert that the fields managed by the form are zeroed out
	assert.Empty(t, config.Addresses)
	assert.Empty(t, config.Links)
	assert.Empty(t, config.Routes)
	assert.Empty(t, config.Hostnames)
	assert.Empty(t, config.Resolvers)
	assert.Empty(t, config.TimeServers)
	assert.Empty(t, config.Operators)

	// assert that the fields not managed by the form are untouched
	assert.Equal(t, base.ExternalIPs, config.ExternalIPs)
	assert.Equal(t, base.Metadata, config.Metadata)
}

func TestFilledFormNoIface(t *testing.T) {
	formData := dashboard.NetworkConfigFormData{
		Base: runtime.PlatformNetworkConfig{
			Metadata: &runtimeres.PlatformMetadataSpec{
				Platform: "foobar",
			},
		},
		Hostname:    "foobar",
		DNSServers:  "1.2.3.4 5.6.7.8",
		TimeServers: "a.example.com   ,  b.example.com",
	}

	config, err := formData.ToPlatformNetworkConfig()
	assert.NoError(t, err)

	assert.Equal(
		t,
		runtime.PlatformNetworkConfig{
			Hostnames: []network.HostnameSpecSpec{{
				Hostname:    "foobar",
				ConfigLayer: network.ConfigPlatform,
			}},
			Resolvers: []network.ResolverSpecSpec{{
				DNSServers: []netip.Addr{
					netip.MustParseAddr("1.2.3.4"),
					netip.MustParseAddr("5.6.7.8"),
				},
				ConfigLayer: network.ConfigPlatform,
			}},
			TimeServers: []network.TimeServerSpecSpec{
				{
					NTPServers:  []string{"a.example.com", "b.example.com"},
					ConfigLayer: network.ConfigPlatform,
				},
			},
			Metadata: &runtimeres.PlatformMetadataSpec{
				Platform: "foobar",
			},
		},
		*config,
	)
}

func TestFilledFormModeDHCP(t *testing.T) {
	formData := dashboard.NetworkConfigFormData{
		Iface: "eth0",
		Mode:  dashboard.ModeDHCP,
	}

	config, err := formData.ToPlatformNetworkConfig()
	assert.NoError(t, err)

	assert.Equal(t, runtime.PlatformNetworkConfig{
		Links: []network.LinkSpecSpec{
			{
				Name:        formData.Iface,
				Logical:     false,
				Up:          true,
				Type:        nethelpers.LinkEther,
				ConfigLayer: network.ConfigPlatform,
			},
		},
		Operators: []network.OperatorSpecSpec{
			{
				Operator:  network.OperatorDHCP4,
				LinkName:  formData.Iface,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric: 1024,
				},
				ConfigLayer: network.ConfigPlatform,
			},
		},
	}, *config)
}

func TestFilledFormModeStaticNoAddresses(t *testing.T) {
	formData := dashboard.NetworkConfigFormData{
		Iface: "eth0",
		Mode:  dashboard.ModeStatic,
	}

	_, err := formData.ToPlatformNetworkConfig()
	assert.ErrorContains(t, err, "no addresses specified")
}

func TestFilledFormModeStaticNoGateway(t *testing.T) {
	formData := dashboard.NetworkConfigFormData{
		Iface:     "eth0",
		Mode:      dashboard.ModeStatic,
		Addresses: "1.2.3.4/24",
	}

	_, err := formData.ToPlatformNetworkConfig()
	assert.ErrorContains(t, err, "unable to parse")
}

func TestFilledFormModeStatic(t *testing.T) {
	formData := dashboard.NetworkConfigFormData{
		Iface:     "eth42",
		Mode:      dashboard.ModeStatic,
		Addresses: "1.2.3.4/24 2.3.4.5/32",
		Gateway:   "3.4.5.6",
	}

	config, err := formData.ToPlatformNetworkConfig()
	assert.NoError(t, err)

	assert.Equal(t, runtime.PlatformNetworkConfig{
		Links: []network.LinkSpecSpec{
			{
				Name:        formData.Iface,
				Logical:     false,
				Up:          true,
				Type:        nethelpers.LinkEther,
				ConfigLayer: network.ConfigPlatform,
			},
		},
		Addresses: []network.AddressSpecSpec{
			{
				Address:     netip.MustParsePrefix("1.2.3.4/24"),
				LinkName:    formData.Iface,
				Family:      nethelpers.FamilyInet4,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				ConfigLayer: network.ConfigPlatform,
			},
			{
				Address:     netip.MustParsePrefix("2.3.4.5/32"),
				LinkName:    formData.Iface,
				Family:      nethelpers.FamilyInet4,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				ConfigLayer: network.ConfigPlatform,
			},
		},
		Routes: []network.RouteSpecSpec{
			{
				Family:      nethelpers.FamilyInet4,
				Gateway:     netip.MustParseAddr("3.4.5.6"),
				OutLinkName: "eth42",
				Table:       nethelpers.TableMain,
				Scope:       nethelpers.ScopeGlobal,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolStatic,
				ConfigLayer: network.ConfigPlatform,
			},
		},
	}, *config)
}
