// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type RouteStatusSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *RouteStatusSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.RouteStatusController{}))

	suite.startRuntime()
}

func (suite *RouteStatusSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *RouteStatusSuite) assertRoutes(requiredIDs []string, check func(*network.RouteStatus, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check)
}

func (suite *RouteStatusSuite) TestRoutes() {
	suite.assertRoutes(
		[]string{"local/inet4//127.0.0.0/8/0"}, func(r *network.RouteStatus, asrt *assert.Assertions) {
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

func (suite *RouteStatusSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestRouteStatusSuite(t *testing.T) {
	suite.Run(t, new(RouteStatusSuite))
}
