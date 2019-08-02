/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package event_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/internal/event"
)

type EventSuite struct {
	suite.Suite
}

func (suite *EventSuite) TestBus() {
	// publish event without subscribers
	event.Bus().Publish(event.Shutdown)

	subscriber1 := make(chan event.Type, 1)
	subscriber2 := make(chan event.Type, 1)

	event.Bus().Subscribe(subscriber1)
	defer event.Bus().Unsubscribe(subscriber1)

	event.Bus().Subscribe(subscriber2)
	defer event.Bus().Unsubscribe(subscriber2)

	select {
	case <-subscriber1:
		suite.Require().Fail("no previous messages should be delivered")
	default:
	}

	// test fan-out
	event.Bus().Publish(event.Reboot)

	suite.Assert().Equal(event.Reboot, <-subscriber1)
	suite.Assert().Equal(event.Reboot, <-subscriber2)

	event.Bus().Unsubscribe(subscriber2)

	event.Bus().Publish(event.Upgrade)

	select {
	case <-subscriber2:
		suite.Require().Fail("message to subscriber2 should not be delivered")
	default:
	}
	suite.Assert().Equal(event.Upgrade, <-subscriber1)
}

func TestEventSuite(t *testing.T) {
	suite.Run(t, new(EventSuite))
}
