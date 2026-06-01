// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type OperatorMergeSuite struct {
	ctest.DefaultSuite
}

func (suite *OperatorMergeSuite) assertOperators(requiredIDs []string, check func(*network.OperatorSpec, *assert.Assertions)) {
	ctest.AssertResources(suite, requiredIDs, check)
}

func (suite *OperatorMergeSuite) assertNoOperator(id string) {
	ctest.AssertNoResource[*network.OperatorSpec](suite, id)
}

func (suite *OperatorMergeSuite) TestMerge() {
	dhcp1 := network.NewOperatorSpec(network.ConfigNamespaceName, "default/dhcp4/eth0")
	*dhcp1.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		ConfigLayer: network.ConfigDefault,
	}

	dhcp2 := network.NewOperatorSpec(network.ConfigNamespaceName, "configuration/dhcp4/eth0")
	*dhcp2.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	dhcp6 := network.NewOperatorSpec(network.ConfigNamespaceName, "configuration/dhcp6/eth0")
	*dhcp6.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP6,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{dhcp1, dhcp2, dhcp6} {
		suite.Create(res)
	}

	suite.assertOperators(
		[]string{
			"dhcp4/eth0",
			"dhcp6/eth0",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "dhcp4/eth0":
				asrt.Equal(*dhcp2.TypedSpec(), *r.TypedSpec())
			case "dhcp6/eth0":
				asrt.Equal(*dhcp6.TypedSpec(), *r.TypedSpec())
			}
		},
	)

	suite.Destroy(dhcp6)

	suite.assertOperators(
		[]string{
			"dhcp4/eth0",
		}, func(r *network.OperatorSpec, asrt *assert.Assertions) {
			asrt.Equal(*dhcp2.TypedSpec(), *r.TypedSpec())
		},
	)
	suite.assertNoOperator("dhcp6/eth0")
}

func (suite *OperatorMergeSuite) TestMergeFlapping() {
	// simulate two conflicting operator definitions which are getting removed/added constantly
	dhcp := network.NewOperatorSpec(network.ConfigNamespaceName, "default/dhcp4/eth0")
	*dhcp.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		ConfigLayer: network.ConfigDefault,
	}

	override := network.NewOperatorSpec(network.ConfigNamespaceName, "configuration/dhcp4/eth0")
	*override.TypedSpec() = network.OperatorSpecSpec{
		Operator:    network.OperatorDHCP4,
		LinkName:    "eth0",
		RequireUp:   true,
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	testMergeFlapping(&suite.DefaultSuite, []*network.OperatorSpec{dhcp, override}, "dhcp4/eth0", override)
}

func TestOperatorMergeSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &OperatorMergeSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(netctrl.NewOperatorMergeController()))
			},
		},
	})
}
