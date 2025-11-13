// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type HardwareAddrSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *HardwareAddrSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HardwareAddrController{}))

	suite.startRuntime()
}

func (suite *HardwareAddrSuite) startRuntime() {
	suite.wg.Go(func() {
		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	})
}

func (suite *HardwareAddrSuite) assertHWAddr(requiredIDs []string, check func(*network.HardwareAddr, *assert.Assertions)) {
	assertResources(suite.ctx, suite.T(), suite.state, requiredIDs, check)
}

func (suite *HardwareAddrSuite) assertNoHWAddr(id string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.HardwareAddrType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		if res.Metadata().ID() == id {
			return retry.ExpectedErrorf("interface %q is still there", id)
		}
	}

	return nil
}

func (suite *HardwareAddrSuite) TestFirst() {
	mustParseMAC := func(addr string) nethelpers.HardwareAddr {
		mac, err := net.ParseMAC(addr)
		suite.Require().NoError(err)

		return nethelpers.HardwareAddr(mac)
	}

	eth0 := network.NewLinkStatus(network.NamespaceName, "eth0")
	eth0.TypedSpec().Type = nethelpers.LinkEther
	eth0.TypedSpec().HardwareAddr = mustParseMAC("56:a0:a0:87:1c:fa")

	eth1 := network.NewLinkStatus(network.NamespaceName, "eth1")
	eth1.TypedSpec().Type = nethelpers.LinkEther
	eth1.TypedSpec().HardwareAddr = mustParseMAC("6a:2b:bd:b2:fc:e0")

	bond0 := network.NewLinkStatus(network.NamespaceName, "bond0")
	bond0.TypedSpec().Type = nethelpers.LinkEther
	bond0.TypedSpec().Kind = "bond"
	bond0.TypedSpec().HardwareAddr = mustParseMAC("56:a0:a0:87:1c:fb")

	suite.Require().NoError(suite.state.Create(suite.ctx, bond0))
	suite.Require().NoError(suite.state.Create(suite.ctx, eth1))

	suite.assertHWAddr(
		[]string{network.FirstHardwareAddr}, func(r *network.HardwareAddr, asrt *assert.Assertions) {
			asrt.Equal(eth1.Metadata().ID(), r.TypedSpec().Name)
			asrt.Equal("6a:2b:bd:b2:fc:e0", net.HardwareAddr(r.TypedSpec().HardwareAddr).String())
		},
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, eth0))

	suite.assertHWAddr(
		[]string{network.FirstHardwareAddr}, func(r *network.HardwareAddr, asrt *assert.Assertions) {
			asrt.Equal(eth0.Metadata().ID(), r.TypedSpec().Name)
			asrt.Equal("56:a0:a0:87:1c:fa", net.HardwareAddr(r.TypedSpec().HardwareAddr).String())
		},
	)

	suite.Require().NoError(suite.state.Destroy(suite.ctx, eth0.Metadata()))
	suite.Require().NoError(suite.state.Destroy(suite.ctx, eth1.Metadata()))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertNoHWAddr(network.FirstHardwareAddr)
			},
		),
	)
}

func (suite *HardwareAddrSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestHardwareAddrSuite(t *testing.T) {
	suite.Run(t, new(HardwareAddrSuite))
}
