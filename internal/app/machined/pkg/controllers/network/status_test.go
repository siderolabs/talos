// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package network_test

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	netctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/resources/files"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type StatusSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *StatusSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&netctrl.StatusController{}))

	suite.startRuntime()
}

func (suite *StatusSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *StatusSuite) assertStatus(expected network.StatusSpec) error {
	status, err := suite.state.Get(suite.ctx, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
	if err != nil {
		if !state.IsNotFoundError(err) {
			suite.Require().NoError(err)
		}

		return retry.ExpectedError(err)
	}

	if *status.(*network.Status).TypedSpec() != expected {
		return retry.ExpectedErrorf("status %+v != expected %+v", *status.(*network.Status).TypedSpec(), expected)
	}

	return nil
}

func (suite *StatusSuite) TestNone() {
	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertStatus(network.StatusSpec{})
		}))
}

func (suite *StatusSuite) TestAddresses() {
	nodeAddress := network.NewNodeAddress(network.NamespaceName, network.NodeAddressCurrentID)
	nodeAddress.TypedSpec().Addresses = []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.0.0.1/24")}

	suite.Require().NoError(suite.state.Create(suite.ctx, nodeAddress))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertStatus(network.StatusSpec{AddressReady: true})
		}))
}

func (suite *StatusSuite) TestRoutes() {
	route := network.NewRouteStatus(network.NamespaceName, "foo")
	route.TypedSpec().Gateway = netaddr.MustParseIP("10.0.0.1")

	suite.Require().NoError(suite.state.Create(suite.ctx, route))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertStatus(network.StatusSpec{ConnectivityReady: true})
		}))
}

func (suite *StatusSuite) TestHostname() {
	hostname := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostname.TypedSpec().Hostname = "foo"

	suite.Require().NoError(suite.state.Create(suite.ctx, hostname))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertStatus(network.StatusSpec{HostnameReady: true})
		}))
}

func (suite *StatusSuite) TestEtcFiles() {
	hosts := files.NewEtcFileStatus(files.NamespaceName, "hosts")
	resolv := files.NewEtcFileStatus(files.NamespaceName, "resolv.conf")

	suite.Require().NoError(suite.state.Create(suite.ctx, hosts))
	suite.Require().NoError(suite.state.Create(suite.ctx, resolv))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			return suite.assertStatus(network.StatusSpec{EtcFilesReady: true})
		}))
}

func (suite *StatusSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()

	// trigger updates in resources to stop watch loops
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewNodeAddress(network.NamespaceName, "bar")))
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewResolverStatus(network.NamespaceName, "bar")))
	suite.Assert().NoError(suite.state.Create(context.Background(), network.NewHostnameStatus(network.NamespaceName, "bar")))
	suite.Assert().NoError(suite.state.Create(context.Background(), files.NewEtcFileStatus(files.NamespaceName, "bar")))
}

func TestStatusSuite(t *testing.T) {
	suite.Run(t, new(StatusSuite))
}
