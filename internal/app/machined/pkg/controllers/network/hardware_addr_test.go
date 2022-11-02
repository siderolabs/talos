// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"fmt"
	"log"
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
	"github.com/stretchr/testify/suite"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/logging"
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

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.HardwareAddrController{}))

	suite.startRuntime()
}

func (suite *HardwareAddrSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *HardwareAddrSuite) assertHWAddr(requiredIDs []string, check func(*network.HardwareAddr) error) error {
	missingIDs := make(map[string]struct{}, len(requiredIDs))

	for _, id := range requiredIDs {
		missingIDs[id] = struct{}{}
	}

	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(network.NamespaceName, network.HardwareAddrType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	for _, res := range resources.Items {
		_, required := missingIDs[res.Metadata().ID()]
		if !required {
			continue
		}

		delete(missingIDs, res.Metadata().ID())

		if err = check(res.(*network.HardwareAddr)); err != nil {
			return retry.ExpectedError(err)
		}
	}

	if len(missingIDs) > 0 {
		return retry.ExpectedError(fmt.Errorf("some resources are missing: %q", missingIDs))
	}

	return nil
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
			return retry.ExpectedError(fmt.Errorf("interface %q is still there", id))
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

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHWAddr(
					[]string{network.FirstHardwareAddr}, func(r *network.HardwareAddr) error {
						if r.TypedSpec().Name != eth1.Metadata().ID() && net.HardwareAddr(r.TypedSpec().HardwareAddr).String() != "6a:2b:bd:b2:fc:e0" {
							return retry.ExpectedErrorf("should be eth1")
						}

						return nil
					},
				)
			},
		),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, eth0))

	suite.Assert().NoError(
		retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertHWAddr(
					[]string{network.FirstHardwareAddr}, func(r *network.HardwareAddr) error {
						if r.TypedSpec().Name != eth0.Metadata().ID() && net.HardwareAddr(r.TypedSpec().HardwareAddr).String() != "56:a0:a0:87:1c:fa" {
							return retry.ExpectedErrorf("should be eth0")
						}

						return nil
					},
				)
			},
		),
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

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(
		suite.state.Create(
			context.Background(),
			network.NewLinkStatus(network.NamespaceName, "bar"),
		),
	)
}

func TestHardwareAddrSuite(t *testing.T) {
	suite.Run(t, new(HardwareAddrSuite))
}
