// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type PlatformConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *PlatformConfigSuite) TestNoPlatform() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.PlatformConfigController{}))

	ctest.AssertNoResource[*network.PlatformConfig](suite, network.PlatformConfigActiveID)
}

func (suite *PlatformConfigSuite) TestPlatform() {
	suite.Require().NoError(
		suite.Runtime().RegisterController(
			&netctrl.PlatformConfigController{
				V1alpha1Platform: &platformMock{
					hostname: []byte("talos-e2e-897b4e49-gcp-controlplane-jvcnl.c.talos-testbed.internal"),
					addresses: []netip.Prefix{
						netip.MustParsePrefix("192.168.1.24/24"),
						netip.MustParsePrefix("2001:fd::3/64"),
					},
					defaultRoutes: []netip.Addr{netip.MustParseAddr("10.0.0.1")},
					linksUp:       []string{"eth0", "eth1"},
					dhcp4Links:    []string{"eth1", "eth2"},
					resolvers:     []netip.Addr{netip.MustParseAddr("1.1.1.1")},
					timeServers:   []string{"pool.ntp.org"},
					tcpProbes:     []string{"example.com:80", "example.com:443"},
					externalIPs: []netip.Addr{
						netip.MustParseAddr("10.3.4.5"),
						netip.MustParseAddr("2001:470:6d:30e:96f4:4219:5733:b860"),
					},
					metadata: &runtimeres.PlatformMetadataSpec{
						Platform: "mock",
						Zone:     "mock-zone",
					},
				},
			},
		),
	)

	ctest.AssertResource(suite, network.PlatformConfigActiveID, func(cfg *network.PlatformConfig, asrt *assert.Assertions) {
		spec := cfg.TypedSpec()

		asrt.Equal(
			[]string{"talos-e2e-897b4e49-gcp-controlplane-jvcnl.c.talos-testbed.internal"},
			xslices.Map(spec.Hostnames, func(h network.HostnameSpecSpec) string {
				return h.FQDN()
			}),
		)
		asrt.Equal(
			[]string{"192.168.1.24/24", "2001:fd::3/64"},
			xslices.Map(spec.Addresses, func(a network.AddressSpecSpec) string {
				return a.Address.String()
			}),
		)
		asrt.Equal(
			[]string{"10.0.0.1"},
			xslices.Map(spec.Routes, func(r network.RouteSpecSpec) string {
				return r.Gateway.String()
			}),
		)
		asrt.Equal(
			[]string{"eth0", "eth1"},
			xslices.Map(spec.Links, func(l network.LinkSpecSpec) string {
				return l.Name
			}),
		)
		asrt.Equal(
			[]string{"eth1", "eth2"},
			xslices.Map(spec.Operators, func(l network.OperatorSpecSpec) string {
				return l.LinkName
			}),
		)
		asrt.Equal(
			[]string{"1.1.1.1"},
			xslices.Map(spec.Resolvers, func(r network.ResolverSpecSpec) string {
				return strings.Join(xslices.Map(r.DNSServers, netip.Addr.String), ", ")
			}),
		)
		asrt.Equal(
			[]string{"pool.ntp.org"},
			xslices.Map(spec.TimeServers, func(t network.TimeServerSpecSpec) string {
				return strings.Join(t.NTPServers, ", ")
			}),
		)
		asrt.Equal(
			[]string{"example.com:80", "example.com:443"},
			xslices.Map(spec.Probes, func(p network.ProbeSpecSpec) string {
				return p.TCP.Endpoint
			}),
		)
		asrt.Equal(
			[]string{"10.3.4.5", "2001:470:6d:30e:96f4:4219:5733:b860"},
			xslices.Map(spec.ExternalIPs, netip.Addr.String),
		)
		asrt.Equal(
			"mock",
			spec.Metadata.Platform,
		)
		asrt.Equal(
			"mock-zone",
			spec.Metadata.Zone,
		)
	})
}

func TestPlatformConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &PlatformConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}

type platformMock struct {
	noData bool

	hostname      []byte
	externalIPs   []netip.Addr
	addresses     []netip.Prefix
	defaultRoutes []netip.Addr
	linksUp       []string
	resolvers     []netip.Addr
	timeServers   []string
	dhcp4Links    []string
	tcpProbes     []string

	metadata *runtimeres.PlatformMetadataSpec
}

func (mock *platformMock) Name() string {
	return "mock"
}

func (mock *platformMock) Configuration(context.Context, state.State) ([]byte, error) {
	return nil, nil
}

func (mock *platformMock) Metadata(context.Context, state.State) (runtimeres.PlatformMetadataSpec, error) {
	return runtimeres.PlatformMetadataSpec{Platform: mock.Name()}, nil
}

func (mock *platformMock) Mode() v1alpha1runtime.Mode {
	return v1alpha1runtime.ModeCloud
}

func (mock *platformMock) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return nil
}

//nolint:gocyclo
func (mock *platformMock) NetworkConfiguration(
	ctx context.Context,
	st state.State,
	ch chan<- *v1alpha1runtime.PlatformNetworkConfig,
) error {
	if mock.noData {
		return nil
	}

	networkConfig := &v1alpha1runtime.PlatformNetworkConfig{
		ExternalIPs: mock.externalIPs,
	}

	if mock.hostname != nil {
		hostnameSpec := network.HostnameSpecSpec{}
		if err := hostnameSpec.ParseFQDN(string(mock.hostname)); err != nil {
			return err
		}

		networkConfig.Hostnames = []network.HostnameSpecSpec{hostnameSpec}
	}

	for _, addr := range mock.addresses {
		family := nethelpers.FamilyInet4
		if addr.Addr().Is6() {
			family = nethelpers.FamilyInet6
		}

		networkConfig.Addresses = append(
			networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     addr,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      family,
			},
		)
	}

	for _, gw := range mock.defaultRoutes {
		family := nethelpers.FamilyInet4
		if gw.Is6() {
			family = nethelpers.FamilyInet6
		}

		route := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     gw,
			OutLinkName: "eth0",
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      family,
			Priority:    1024,
		}

		route.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, route)
	}

	for _, link := range mock.linksUp {
		networkConfig.Links = append(
			networkConfig.Links, network.LinkSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				Name:        link,
				Up:          true,
			},
		)
	}

	if len(mock.resolvers) > 0 {
		networkConfig.Resolvers = append(
			networkConfig.Resolvers, network.ResolverSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				DNSServers:  mock.resolvers,
			},
		)
	}

	if len(mock.timeServers) > 0 {
		networkConfig.TimeServers = append(
			networkConfig.TimeServers, network.TimeServerSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				NTPServers:  mock.timeServers,
			},
		)
	}

	for _, link := range mock.dhcp4Links {
		networkConfig.Operators = append(
			networkConfig.Operators, network.OperatorSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    link,
				Operator:    network.OperatorDHCP4,
				DHCP4:       network.DHCP4OperatorSpec{},
			},
		)
	}

	for _, endpoint := range mock.tcpProbes {
		networkConfig.Probes = append(
			networkConfig.Probes, network.ProbeSpecSpec{
				Interval: time.Second,
				TCP: network.TCPProbeSpec{
					Endpoint: endpoint,
					Timeout:  time.Second,
				},
				ConfigLayer: network.ConfigPlatform,
			})
	}

	networkConfig.Metadata = mock.metadata

	for range 5 { // send the network config multiple times to test duplicate suppression
		select {
		case ch <- networkConfig:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
