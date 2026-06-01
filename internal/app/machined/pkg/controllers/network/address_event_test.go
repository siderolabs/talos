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

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
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

func (s *mockEventsStream) lastAddressEvent() *machine.AddressEvent {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

	if len(s.messages) == 0 {
		return nil
	}

	event, _ := s.messages[len(s.messages)-1].(*machine.AddressEvent)

	return event
}

type AddressEventsSuite struct {
	ctest.DefaultSuite

	events *mockEventsStream
}

func (suite *AddressEventsSuite) TestReconcile() {
	hostname := networkresource.NewHostnameStatus(networkresource.NamespaceName, networkresource.HostnameID)
	hostname.TypedSpec().Hostname = "localhost"

	suite.Create(hostname)

	var event *machine.AddressEvent

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		event = suite.events.lastAddressEvent()

		if event == nil {
			return retry.ExpectedErrorf("no address event created")
		}

		if event.Hostname == "" {
			return retry.ExpectedErrorf("expected hostname to be set")
		}

		return nil
	})

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

	suite.Create(nodeAddress)

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		event = suite.events.lastAddressEvent()

		if event == nil {
			return errors.New("no address event created")
		}

		if len(event.Addresses) == 0 {
			return retry.ExpectedErrorf("expected addresses to be set")
		}

		return nil
	})

	suite.Require().Equal(addrs, event.Addresses)
}

func TestAddressEventsSuite(t *testing.T) {
	t.Parallel()

	addrSuite := &AddressEventsSuite{}
	addrSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			addrSuite.events = &mockEventsStream{
				messages: []proto.Message{},
			}

			s.Require().NoError(s.Runtime().RegisterController(&network.AddressEventController{
				V1Alpha1Events: addrSuite.events,
			}))
		},
	}

	suite.Run(t, addrSuite)
}
