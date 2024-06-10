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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type TimeServerMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *TimeServerMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.TimeServerMergeController{}))

	suite.startRuntime()
}

func (suite *TimeServerMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *TimeServerMergeSuite) assertTimeServers(
	requiredIDs []string,
	check func(*network.TimeServerSpec, *assert.Assertions),
) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check)
}

func (suite *TimeServerMergeSuite) TestMerge() {
	def := network.NewTimeServerSpec(network.ConfigNamespaceName, "default/timeservers")
	*def.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{constants.DefaultNTPServer},
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewTimeServerSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"ntp.eth0"},
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewTimeServerSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"ntp.eth1"},
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewTimeServerSpec(network.ConfigNamespaceName, "configuration/timeservers")
	*static.TypedSpec() = network.TimeServerSpecSpec{
		NTPServers:  []string{"my.ntp"},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.assertTimeServers(
		[]string{
			"timeservers",
		}, func(r *network.TimeServerSpec, asrt *assert.Assertions) {
			asrt.Equal(*static.TypedSpec(), *r.TypedSpec())
		},
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.assertTimeServers(
		[]string{
			"timeservers",
		}, func(r *network.TimeServerSpec, asrt *assert.Assertions) {
			asrt.Equal([]string{"ntp.eth0", "ntp.eth1"}, r.TypedSpec().NTPServers)
		},
	)
}

func (suite *TimeServerMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestTimeServerMergeSuite(t *testing.T) {
	suite.Run(t, new(TimeServerMergeSuite))
}
