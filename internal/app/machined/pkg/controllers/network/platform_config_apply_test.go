// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type PlatformConfigApplySuite struct {
	ctest.DefaultSuite
}

func (suite *PlatformConfigApplySuite) TestHostname() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigCachedID)
	platformConfig.TypedSpec().Hostnames = []network.HostnameSpecSpec{
		{
			Hostname:    "talos-e2e-897b4e49-gcp-controlplane-jvcnl",
			Domainname:  "c.talos-testbed.internal",
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResource(suite, "platform/hostname", func(hostname *network.HostnameSpec, asrt *assert.Assertions) {
		spec := hostname.TypedSpec()

		asrt.Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
		asrt.Equal("c.talos-testbed.internal", spec.Domainname)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestHostnameNoDomain() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Hostnames = []network.HostnameSpecSpec{
		{
			Hostname:    "talos-e2e-897b4e49-gcp-controlplane-jvcnl",
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResource(suite, "platform/hostname", func(hostname *network.HostnameSpec, asrt *assert.Assertions) {
		spec := hostname.TypedSpec()

		asrt.Equal("talos-e2e-897b4e49-gcp-controlplane-jvcnl", spec.Hostname)
		asrt.Equal("", spec.Domainname)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestAddresses() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Addresses = []network.AddressSpecSpec{
		{
			Address:     netip.MustParsePrefix("192.168.1.24/24"),
			LinkName:    "eth0",
			Family:      nethelpers.FamilyInet4,
			ConfigLayer: network.ConfigPlatform,
		},
		{
			Address:     netip.MustParsePrefix("2001:fd::3/64"),
			LinkName:    "eth0",
			Family:      nethelpers.FamilyInet6,
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"platform/eth0/192.168.1.24/24",
		"platform/eth0/2001:fd::3/64",
	}, func(r *network.AddressSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		switch r.Metadata().ID() {
		case "platform/eth0/192.168.1.24/24":
			asrt.Equal(nethelpers.FamilyInet4, spec.Family)
			asrt.Equal("192.168.1.24/24", spec.Address.String())
		case "platform/eth0/2001:fd::3/64":
			asrt.Equal(nethelpers.FamilyInet6, spec.Family)
			asrt.Equal("2001:fd::3/64", spec.Address.String())
		}

		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestLinks() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Links = []network.LinkSpecSpec{
		{
			Name:        "eth0",
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		},
		{
			Name:        "eth1",
			Up:          true,
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"platform/eth0",
		"platform/eth1",
	}, func(r *network.LinkSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.True(spec.Up)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestRoutes() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Routes = []network.RouteSpecSpec{
		{
			Family:      nethelpers.FamilyInet4,
			Gateway:     netip.MustParseAddr("10.0.0.1"),
			OutLinkName: "eth0",
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Priority:    1024,
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"platform/inet4/10.0.0.1//1024",
	}, func(r *network.RouteSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("10.0.0.1", spec.Gateway.String())
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestOperators() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Operators = []network.OperatorSpecSpec{
		{
			ConfigLayer: network.ConfigPlatform,
			LinkName:    "eth1",
			Operator:    network.OperatorDHCP4,
			DHCP4:       network.DHCP4OperatorSpec{},
		},
		{
			ConfigLayer: network.ConfigPlatform,
			LinkName:    "eth2",
			Operator:    network.OperatorDHCP4,
			DHCP4:       network.DHCP4OperatorSpec{},
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"platform/dhcp4/eth1",
		"platform/dhcp4/eth2",
	}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(network.OperatorDHCP4, spec.Operator)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestResolvers() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Resolvers = []network.ResolverSpecSpec{
		{
			DNSServers:  []netip.Addr{netip.MustParseAddr("1.1.1.1")},
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"platform/resolvers",
	}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("[1.1.1.1]", fmt.Sprintf("%s", spec.DNSServers))
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestTimeServers() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().TimeServers = []network.TimeServerSpecSpec{
		{
			NTPServers:  []string{"pool.ntp.org"},
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"platform/timeservers",
	}, func(r *network.TimeServerSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("[pool.ntp.org]", fmt.Sprintf("%s", spec.NTPServers))
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *PlatformConfigApplySuite) TestProbes() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Probes = []network.ProbeSpecSpec{
		{
			Interval: time.Second,
			TCP: network.TCPProbeSpec{
				Endpoint: "example.com:80",
				Timeout:  time.Second,
			},
			ConfigLayer: network.ConfigPlatform,
		},
		{
			Interval: time.Second,
			TCP: network.TCPProbeSpec{
				Endpoint: "example.com:443",
				Timeout:  time.Second,
			},
			ConfigLayer: network.ConfigPlatform,
		},
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"tcp:example.com:80",
		"tcp:example.com:443",
	}, func(r *network.ProbeSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(time.Second, spec.Interval)
		asrt.Equal(network.ConfigPlatform, spec.ConfigLayer)
	})
}

func (suite *PlatformConfigApplySuite) TestExternalIPs() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().ExternalIPs = []netip.Addr{
		netip.MustParseAddr("10.3.4.5"),
		netip.MustParseAddr("2001:470:6d:30e:96f4:4219:5733:b860"),
	}
	suite.Create(platformConfig)

	ctest.AssertResources(suite, []string{
		"external/10.3.4.5/32",
		"external/2001:470:6d:30e:96f4:4219:5733:b860/128",
	}, func(r *network.AddressStatus, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("external", spec.LinkName)
		asrt.Equal(nethelpers.ScopeGlobal, spec.Scope)

		if r.Metadata().ID() == "external/10.3.4.5/32" {
			asrt.Equal(nethelpers.FamilyInet4, spec.Family)
		} else {
			asrt.Equal(nethelpers.FamilyInet6, spec.Family)
		}
	})
}

func (suite *PlatformConfigApplySuite) TestMetadata() {
	platformConfig := network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID)
	platformConfig.TypedSpec().Metadata = &runtimeres.PlatformMetadataSpec{
		Platform: "mock",
		Zone:     "mock-zone",
	}
	suite.Create(platformConfig)

	ctest.AssertResource(suite, runtimeres.PlatformMetadataID,
		func(r *runtimeres.PlatformMetadata, asrt *assert.Assertions) {
			asrt.Equal("mock", r.TypedSpec().Platform)
			asrt.Equal("mock-zone", r.TypedSpec().Zone)
		})
}

func TestPlatformConfigApplySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &PlatformConfigApplySuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(
					suite.Runtime().RegisterController(
						&netctrl.PlatformConfigApplyController{},
					))
			},
		},
	})
}
