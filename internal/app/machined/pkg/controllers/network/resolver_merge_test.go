// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ResolverMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *ResolverMergeSuite) assertResolvers(requiredIDs []string, check func(*network.ResolverSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *ResolverMergeSuite) TestMerge() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)},
			{Addr: netip.MustParseAddr(constants.DefaultSecondaryResolver)},
		},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("1.1.2.0")},
		},
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("1.1.2.1")},
		},
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewResolverSpec(network.ConfigNamespaceName, "configuration/resolvers")
	*static.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("2.2.2.2")},
		},
		SearchDomains: []string{"example.com", "example.org", "example.net"},
		ConfigLayer:   network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Create(res)
	}

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]network.NameServerSpec{
					{Addr: netip.MustParseAddr("2.2.2.2")},
				}, r.TypedSpec().NameServers,
			)
			asrt.Equal(
				[]netip.Addr{netip.MustParseAddr("2.2.2.2")}, r.TypedSpec().DNSServers, //nolint:staticcheck
			)
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
			asrt.Equal([]string{"example.com", "example.org", "example.net"}, r.TypedSpec().SearchDomains)
		},
	)

	suite.Destroy(static)

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(
				[]network.NameServerSpec{
					{Addr: netip.MustParseAddr("1.1.2.0")},
					{Addr: netip.MustParseAddr("1.1.2.1")},
				}, r.TypedSpec().NameServers,
			)
			asrt.Equal([]netip.Addr{netip.MustParseAddr("1.1.2.0"), netip.MustParseAddr("1.1.2.1")}, r.TypedSpec().DNSServers) //nolint:staticcheck
		},
	)
}

func (suite *ResolverMergeSuite) TestMergeIPv46() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)},
			{Addr: netip.MustParseAddr(constants.DefaultSecondaryResolver)},
		},
		ConfigLayer: network.ConfigDefault,
	}

	platform := network.NewResolverSpec(network.ConfigNamespaceName, "platform/resolvers")
	*platform.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("1.1.2.0")},
			{Addr: netip.MustParseAddr("fe80::1")},
		},
		ConfigLayer: network.ConfigPlatform,
	}

	dhcp := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("1.1.2.1")},
		},
		ConfigLayer: network.ConfigOperator,
	}

	for _, res := range []resource.Resource{def, platform, dhcp} {
		suite.Create(res)
	}

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigOperator, r.TypedSpec().ConfigLayer)
			asrt.Equal([]network.NameServerSpec{
				{Addr: netip.MustParseAddr("1.1.2.1")},
				{Addr: netip.MustParseAddr("fe80::1")},
			}, r.TypedSpec().NameServers)
			asrt.Equal(`["1.1.2.1" "fe80::1"]`, fmt.Sprintf("%q", r.TypedSpec().DNSServers)) //nolint:staticcheck
		},
	)
}

func (suite *ResolverMergeSuite) TestMergeSearchDomainsOnlyConfig() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)},
			{Addr: netip.MustParseAddr(constants.DefaultSecondaryResolver)},
		},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("192.168.131.1")},
		},
		SearchDomains: []string{"somewhere.com", "home.lab"},
		ConfigLayer:   network.ConfigOperator,
	}

	cfg := network.NewResolverSpec(network.ConfigNamespaceName, "configuration/resolvers")
	*cfg.TypedSpec() = network.ResolverSpecSpec{
		SearchDomains: []string{"home.lab", "another.lab"},
		ConfigLayer:   network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp, cfg} {
		suite.Create(res)
	}

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal([]netip.Addr{netip.MustParseAddr("192.168.131.1")}, r.TypedSpec().DNSServers) //nolint:staticcheck
			asrt.Equal([]network.NameServerSpec{
				{Addr: netip.MustParseAddr("192.168.131.1")},
			}, r.TypedSpec().NameServers)
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
			asrt.Equal([]string{"another.lab", "somewhere.com", "home.lab"}, r.TypedSpec().SearchDomains)
		},
	)
}

func (suite *ResolverMergeSuite) TestMergeIPv6OnlyConfig() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)},
			{Addr: netip.MustParseAddr(constants.DefaultSecondaryResolver)},
		},
		ConfigLayer: network.ConfigDefault,
	}

	cfg := network.NewResolverSpec(network.ConfigNamespaceName, "cfg/resolvers")
	*cfg.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr("fe80::1")},
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, cfg} {
		suite.Create(res)
	}

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
			asrt.Equal([]network.NameServerSpec{
				{Addr: netip.MustParseAddr("fe80::1")},
			}, r.TypedSpec().NameServers)
			asrt.Equal(`["fe80::1"]`, fmt.Sprintf("%q", r.TypedSpec().DNSServers)) //nolint:staticcheck
		},
	)
}

func (suite *ResolverMergeSuite) TestMergeDNSOverTLS() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)},
			{Addr: netip.MustParseAddr(constants.DefaultSecondaryResolver)},
		},
		ConfigLayer: network.ConfigDefault,
	}

	static := network.NewResolverSpec(network.ConfigNamespaceName, "configuration/resolvers")
	*static.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{
			{
				Addr:          netip.MustParseAddr("9.9.9.9"),
				Protocol:      nethelpers.DNSProtocolDNSOverTLS,
				TLSServerName: "dns.quad9.net",
			},
			{
				Addr: netip.MustParseAddr("8.8.8.8"),
			},
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, static} {
		suite.Create(res)
	}

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
			asrt.Equal(
				[]netip.Addr{netip.MustParseAddr("9.9.9.9"), netip.MustParseAddr("8.8.8.8")},
				r.TypedSpec().DNSServers, //nolint:staticcheck
			)
			asrt.Equal(
				[]network.NameServerSpec{
					{
						Addr:          netip.MustParseAddr("9.9.9.9"),
						Protocol:      nethelpers.DNSProtocolDNSOverTLS,
						TLSServerName: "dns.quad9.net",
					},
					{
						Addr: netip.MustParseAddr("8.8.8.8"),
					},
				},
				r.TypedSpec().NameServers,
			)
		},
	)
}

func TestResolverMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ResolverMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewResolverMergeController()))
			},
		},
	})
}
