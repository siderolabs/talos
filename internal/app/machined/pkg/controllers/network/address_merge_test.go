// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type AddressMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *AddressMergeSuite) assertAddresses(requiredIDs []string, check func(*network.AddressSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *AddressMergeSuite) assertNoAddress(id string) {
	ctest.AssertNoResource[*network.AddressSpec](suite, id)
}

func (suite *AddressMergeSuite) TestMerge() {
	loopback := network.NewAddressSpec(network.ConfigNamespaceName, "default/lo/127.0.0.1/8")
	*loopback.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("127.0.0.1/8"),
		LinkName:    "lo",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeHost,
		ConfigLayer: network.ConfigDefault,
	}

	dhcp := network.NewAddressSpec(network.ConfigNamespaceName, "dhcp/eth0/10.0.0.1/8")
	*dhcp.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.1/8"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewAddressSpec(network.ConfigNamespaceName, "configuration/eth0/10.0.0.35/32")
	*static.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.35/32"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	override := network.NewAddressSpec(network.ConfigNamespaceName, "configuration/eth0/10.0.0.1/8")
	*override.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.1/8"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeHost,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{loopback, dhcp, static, override} {
		suite.Create(res)
	}

	suite.assertAddresses(
		[]string{
			"lo/127.0.0.1/8",
			"eth0/10.0.0.1/8",
			"eth0/10.0.0.35/32",
		}, func(r *network.AddressSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "lo/127.0.0.1/8":
				asrt.Equal(*loopback.TypedSpec(), *r.TypedSpec())
			case "eth0/10.0.0.1/8":
				asrt.Equal(*override.TypedSpec(), *r.TypedSpec())
			case "eth0/10.0.0.35/32":
				asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	suite.Destroy(static)

	suite.assertAddresses(
		[]string{
			"lo/127.0.0.1/8",
			"eth0/10.0.0.1/8",
		}, func(*network.AddressSpec, *assert.Assertions) {},
	)

	suite.assertNoAddress("eth0/10.0.0.35/32")
}

func (suite *AddressMergeSuite) TestMergeFlapping() {
	// simulate two conflicting address definitions which are getting removed/added constantly
	dhcp := network.NewAddressSpec(network.ConfigNamespaceName, "dhcp/eth0/10.0.0.1/8")
	*dhcp.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.1/8"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigOperator,
	}

	override := network.NewAddressSpec(network.ConfigNamespaceName, "configuration/eth0/10.0.0.1/8")
	*override.TypedSpec() = network.AddressSpecSpec{
		Address:     netip.MustParsePrefix("10.0.0.1/8"),
		LinkName:    "eth0",
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeHost,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	testMergeFlapping(&suite.DefaultSuite, []*network.AddressSpec{dhcp, override}, "eth0/10.0.0.1/8", override)
}

func TestAddressMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &AddressMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(netctrl.NewAddressMergeController()))
			},
		},
	})
}
