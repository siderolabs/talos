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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HostnameMergeSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *HostnameMergeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HostnameMergeController{}))

	suite.startRuntime()
}

func (suite *HostnameMergeSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *HostnameMergeSuite) assertHostnames(requiredIDs []string, check func(*network.HostnameSpec, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check)
}

func (suite *HostnameMergeSuite) TestMerge() {
	def := network.NewHostnameSpec(network.ConfigNamespaceName, "default/hostname")
	*def.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "foo",
		Domainname:  "tld",
		ConfigLayer: network.ConfigDefault,
	}

	dhcp1 := network.NewHostnameSpec(network.ConfigNamespaceName, "dhcp/eth0")
	*dhcp1.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "eth-0",
		ConfigLayer: network.ConfigOperator,
	}

	dhcp2 := network.NewHostnameSpec(network.ConfigNamespaceName, "dhcp/eth1")
	*dhcp2.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "eth-1",
		ConfigLayer: network.ConfigOperator,
	}

	static := network.NewHostnameSpec(network.ConfigNamespaceName, "configuration/hostname")
	*static.TypedSpec() = network.HostnameSpecSpec{
		Hostname:    "bar",
		Domainname:  "com",
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	for _, res := range []resource.Resource{def, dhcp1, dhcp2, static} {
		suite.Require().NoError(suite.state.Create(suite.ctx, res), "%v", res.Spec())
	}

	suite.assertHostnames(
		[]string{
			"hostname",
		}, func(r *network.HostnameSpec, asrt *assert.Assertions) {
			asrt.Equal("bar.com", r.TypedSpec().FQDN())
			asrt.Equal("bar", r.TypedSpec().Hostname)
			asrt.Equal("com", r.TypedSpec().Domainname)
		},
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, static.Metadata()))

	suite.assertHostnames(
		[]string{
			"hostname",
		}, func(r *network.HostnameSpec, asrt *assert.Assertions) {
			asrt.Equal("eth-0", r.TypedSpec().FQDN())
		},
	)
}

func (suite *HostnameMergeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestHostnameMergeSuite(t *testing.T) {
	suite.Run(t, new(HostnameMergeSuite))
}
