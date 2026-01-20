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
		DNSServers: []netip.Addr{
			netip.MustParseAddr(constants.DefaultPrimaryResolver),
			netip.MustParseAddr(constants.DefaultSecondaryResolver),
		},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{netip.MustParseAddr("1.1.2.0")},
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{netip.MustParseAddr("1.1.2.1")},
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewResolverSpec(network.ConfigNamespaceName, "configuration/resolvers")
	*static.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:    []netip.Addr{netip.MustParseAddr("2.2.2.2")},
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
			asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			asrt.Equal([]string{"example.com", "example.org", "example.net"}, r.TypedSpec().SearchDomains)
		},
	)

	suite.Destroy(static)

	suite.assertResolvers(
		[]string{
			"resolvers",
		}, func(r *network.ResolverSpec, asrt *assert.Assertions) {
			asrt.Equal([]netip.Addr{netip.MustParseAddr("1.1.2.0"), netip.MustParseAddr("1.1.2.1")}, r.TypedSpec().DNSServers)
		},
	)
}

func (suite *ResolverMergeSuite) TestMergeIPv46() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		DNSServers: []netip.Addr{
			netip.MustParseAddr(constants.DefaultPrimaryResolver),
			netip.MustParseAddr(constants.DefaultSecondaryResolver),
		},
		ConfigLayer: network.ConfigDefault,
	}

	platform := network.NewResolverSpec(network.ConfigNamespaceName, "platform/resolvers")
	*platform.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{netip.MustParseAddr("1.1.2.0"), netip.MustParseAddr("fe80::1")},
		ConfigLayer: network.ConfigPlatform,
	}

	dhcp := network.NewResolverSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{netip.MustParseAddr("1.1.2.1")},
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
			asrt.Equal(`["1.1.2.1" "fe80::1"]`, fmt.Sprintf("%q", r.TypedSpec().DNSServers))
		},
	)
}

func (suite *ResolverMergeSuite) TestMergeIPv6OnlyConfig() {
	def := network.NewResolverSpec(network.ConfigNamespaceName, "default/resolvers")
	*def.TypedSpec() = network.ResolverSpecSpec{
		DNSServers: []netip.Addr{
			netip.MustParseAddr(constants.DefaultPrimaryResolver),
			netip.MustParseAddr(constants.DefaultSecondaryResolver),
		},
		ConfigLayer: network.ConfigDefault,
	}

	cfg := network.NewResolverSpec(network.ConfigNamespaceName, "cfg/resolvers")
	*cfg.TypedSpec() = network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{netip.MustParseAddr("fe80::1")},
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
			asrt.Equal(`["fe80::1"]`, fmt.Sprintf("%q", r.TypedSpec().DNSServers))
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
