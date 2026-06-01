// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RoutingRuleStatusSuite struct {
	ctest.DefaultSuite
}

func (suite *RoutingRuleStatusSuite) TestRules() {
	// Every Linux system has default rules:
	//   0:      from all lookup local
	//   32766:  from all lookup main
	//   32767:  from all lookup default
	//
	// Assert that at least the "from all lookup local" rule (priority 0, table 255/local) is published.
	ctest.AssertResource(
		suite,
		"inet4/00000",
		func(r *network.RoutingRuleStatus, asrt *assert.Assertions) {
			asrt.Equal(nethelpers.FamilyInet4, r.TypedSpec().Family)
			asrt.Equal(nethelpers.RoutingTable(255), r.TypedSpec().Table) // local table
			asrt.EqualValues(0, r.TypedSpec().Priority)
			asrt.Equal(nethelpers.RoutingRuleActionUnicast, r.TypedSpec().Action)
		},
	)
}

func TestRoutingRuleStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RoutingRuleStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RoutingRuleStatusController{}))
			},
		},
	})
}
