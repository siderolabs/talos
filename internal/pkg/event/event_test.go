// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package event_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/pkg/event"
)

type EventSuite struct {
	suite.Suite
}

func (suite *EventSuite) TestBus() {
	// publish event without subscribers
	event.Bus().Notify(event.Event{Type: event.Shutdown})

	subscriber1 := struct {
		*event.Embeddable
	}{
		&event.Embeddable{Chan: make(event.Channel, 20)},
	}
	subscriber2 := struct {
		*event.Embeddable
	}{
		&event.Embeddable{Chan: make(event.Channel, 20)},
	}

	event.Bus().Register(subscriber1)
	defer event.Bus().Unregister(subscriber1)

	event.Bus().Register(subscriber2)
	defer event.Bus().Unregister(subscriber2)

	select {
	case <-subscriber1.Channel():
		suite.Require().Fail("no previous messages should be delivered")
	default:
	}

	// test fan-out
	event.Bus().Notify(event.Event{Type: event.Reboot})

	suite.Assert().Equal(event.Event{Type: event.Reboot, Data: nil}, <-subscriber1.Channel())
	suite.Assert().Equal(event.Event{Type: event.Reboot, Data: nil}, <-subscriber2.Channel())

	event.Bus().Unregister(subscriber2)

	event.Bus().Notify(event.Event{Type: event.Upgrade})

	select {
	case <-subscriber2.Channel():
		suite.Require().Fail("message to subscriber2 should not be delivered")
	default:
	}
	suite.Assert().Equal(event.Event{Type: event.Upgrade, Data: nil}, <-subscriber1.Channel())
}

func TestEventSuite(t *testing.T) {
	suite.Run(t, new(EventSuite))
}
