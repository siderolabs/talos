// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/proto"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	networkresource "github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type mockEventsStream struct {
	messages []proto.Message
}

func (s *mockEventsStream) Publish(m proto.Message) {
	s.messages = append(s.messages, m)
}

type EndpointsEventsSuite struct {
	suite.Suite

	events *mockEventsStream
	state  state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *EndpointsEventsSuite) SetupTest() {
	suite.events = &mockEventsStream{
		messages: []proto.Message{},
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&network.AddressEventController{
		V1Alpha1Events: suite.events,
	}))

	suite.startRuntime()
}

func (suite *EndpointsEventsSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *EndpointsEventsSuite) TestReconcile() {
	hostname := networkresource.NewHostnameStatus(networkresource.NamespaceName, networkresource.HostnameID)
	hostname.TypedSpec().Hostname = "localhost"

	suite.Require().NoError(suite.state.Create(suite.ctx, hostname))

	var event *machine.AddressEvent

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			if len(suite.events.messages) == 0 {
				return retry.ExpectedError(fmt.Errorf("no events created"))
			}

			m := suite.events.messages[len(suite.events.messages)-1]

			var ok bool

			event, ok = m.(*machine.AddressEvent)
			if !ok {
				return fmt.Errorf("not an endpoint event")
			}

			if event.Hostname == "" {
				return retry.ExpectedError(fmt.Errorf("expected hostname to be set"))
			}

			return nil
		},
	))

	suite.Require().Equal(hostname.TypedSpec().Hostname, event.Hostname)
	suite.Require().Empty(event.Addresses)

	nodeAddress := networkresource.NewNodeAddress(networkresource.NamespaceName, networkresource.FilteredNodeAddressID(
		networkresource.NodeAddressCurrentID,
		k8s.NodeAddressFilterNoK8s),
	)

	addrs := []string{
		"10.5.0.2",
		"127.0.0.2",
	}

	nodeAddress.TypedSpec().Addresses = append(
		nodeAddress.TypedSpec().Addresses,
		netaddr.IPPrefixFrom(netaddr.MustParseIP(addrs[0]), 32),
		netaddr.IPPrefixFrom(netaddr.MustParseIP(addrs[1]), 32),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, nodeAddress))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			if len(suite.events.messages) == 0 {
				return retry.ExpectedError(fmt.Errorf("no events created"))
			}

			m := suite.events.messages[len(suite.events.messages)-1]

			var ok bool

			event, ok = m.(*machine.AddressEvent)
			if !ok {
				return fmt.Errorf("not an address event")
			}

			if len(event.Addresses) == 0 {
				return retry.ExpectedError(fmt.Errorf("expected addresses to be set"))
			}

			return nil
		},
	))

	suite.Require().Equal(addrs, event.Addresses)
}

func (suite *EndpointsEventsSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestEndpointsEventsSuite(t *testing.T) {
	suite.Run(t, new(EndpointsEventsSuite))
}
