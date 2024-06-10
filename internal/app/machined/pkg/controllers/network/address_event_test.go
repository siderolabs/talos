// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	networkresource "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type mockEventsStream struct {
	messagesMu sync.Mutex
	messages   []proto.Message
}

func (s *mockEventsStream) Publish(_ context.Context, m proto.Message) {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	s.messages = append(s.messages, m)
}

type AddressEventsSuite struct {
	suite.Suite

	events *mockEventsStream
	state  state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *AddressEventsSuite) SetupTest() {
	suite.events = &mockEventsStream{
		messages: []proto.Message{},
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&network.AddressEventController{
				V1Alpha1Events: suite.events,
			},
		),
	)

	suite.startRuntime()
}

func (suite *AddressEventsSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *AddressEventsSuite) TestReconcile() {
	hostname := networkresource.NewHostnameStatus(networkresource.NamespaceName, networkresource.HostnameID)
	hostname.TypedSpec().Hostname = "localhost"

	suite.Require().NoError(suite.state.Create(suite.ctx, hostname))

	var event *machine.AddressEvent

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				suite.events.messagesMu.Lock()
				defer suite.events.messagesMu.Unlock()

				if len(suite.events.messages) == 0 {
					return retry.ExpectedErrorf("no events created")
				}

				m := suite.events.messages[len(suite.events.messages)-1]

				var ok bool

				event, ok = m.(*machine.AddressEvent)
				if !ok {
					return errors.New("not an endpoint event")
				}

				if event.Hostname == "" {
					return retry.ExpectedErrorf("expected hostname to be set")
				}

				return nil
			},
		),
	)

	suite.Require().Equal(hostname.TypedSpec().Hostname, event.Hostname)
	suite.Require().Empty(event.Addresses)

	nodeAddress := networkresource.NewNodeAddress(
		networkresource.NamespaceName, networkresource.FilteredNodeAddressID(
			networkresource.NodeAddressCurrentID,
			k8s.NodeAddressFilterNoK8s,
		),
	)

	addrs := []string{
		"10.5.0.2",
		"127.0.0.2",
	}

	nodeAddress.TypedSpec().Addresses = append(
		nodeAddress.TypedSpec().Addresses,
		netip.PrefixFrom(netip.MustParseAddr(addrs[0]), 32),
		netip.PrefixFrom(netip.MustParseAddr(addrs[1]), 32),
	)

	suite.Require().NoError(suite.state.Create(suite.ctx, nodeAddress))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				suite.events.messagesMu.Lock()
				defer suite.events.messagesMu.Unlock()

				if len(suite.events.messages) == 0 {
					return retry.ExpectedErrorf("no events created")
				}

				m := suite.events.messages[len(suite.events.messages)-1]

				var ok bool

				event, ok = m.(*machine.AddressEvent)
				if !ok {
					return errors.New("not an address event")
				}

				if len(event.Addresses) == 0 {
					return retry.ExpectedErrorf("expected addresses to be set")
				}

				return nil
			},
		),
	)

	suite.Require().Equal(addrs, event.Addresses)
}

func (suite *AddressEventsSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestAddressEventsSuite(t *testing.T) {
	suite.Run(t, new(AddressEventsSuite))
}
