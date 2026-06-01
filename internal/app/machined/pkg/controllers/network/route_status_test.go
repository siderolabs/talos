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

type RouteStatusSuite struct {
	ctest.DefaultSuite
}

func (suite *RouteStatusSuite) TestRoutes() {
	ctest.AssertResource(
		suite,
		"local/inet4//127.0.0.0/8/0",
		func(r *network.RouteStatus, asrt *assert.Assertions) {
			asrt.True(r.TypedSpec().Source.IsLoopback())
			asrt.Equal("lo", r.TypedSpec().OutLinkName)
			asrt.Equal(nethelpers.TableLocal, r.TypedSpec().Table)
			asrt.Equal(nethelpers.ScopeHost, r.TypedSpec().Scope)
			asrt.Equal(nethelpers.TypeLocal, r.TypedSpec().Type)
			asrt.Equal(nethelpers.ProtocolKernel, r.TypedSpec().Protocol)
			asrt.EqualValues(0, r.TypedSpec().MTU)
		},
	)
}

func TestRouteStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &RouteStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.RouteStatusController{}))
			},
		},
	})
}
