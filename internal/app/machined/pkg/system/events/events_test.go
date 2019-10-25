// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
)

type EventsSuite struct {
	suite.Suite
}

func (suite *EventsSuite) assertEvents(expectedMessages []string, evs []events.ServiceEvent) {
	messages := make([]string, len(evs))

	for i := range evs {
		messages[i] = evs[i].Message
	}

	suite.Assert().Equal(expectedMessages, messages)
}

func (suite *EventsSuite) TestEmpty() {
	var e events.ServiceEvents

	suite.Assert().Equal([]events.ServiceEvent(nil), e.Get(100))
}

func (suite *EventsSuite) TestSome() {
	var e events.ServiceEvents

	for i := 0; i < 5; i++ {
		e.Push(events.ServiceEvent{
			Message: strconv.Itoa(i),
		})
	}

	suite.Assert().Equal([]events.ServiceEvent(nil), e.Get(0))
	suite.assertEvents([]string{"4"}, e.Get(1))
	suite.assertEvents([]string{"1", "2", "3", "4"}, e.Get(4))
	suite.assertEvents([]string{"0", "1", "2", "3", "4"}, e.Get(5))
	suite.assertEvents([]string{"0", "1", "2", "3", "4"}, e.Get(6))
	suite.assertEvents([]string{"0", "1", "2", "3", "4"}, e.Get(100))

	protoEvents := e.AsProto(1)
	suite.Assert().Len(protoEvents.Events, 1)
	suite.Assert().Equal("4", protoEvents.Events[0].Msg)
	suite.Assert().Equal("Initialized", protoEvents.Events[0].State)
}

func (suite *EventsSuite) TestOverflow() {
	var e events.ServiceEvents

	numEvents := events.MaxEventsToKeep*2 + 3

	for i := 0; i < numEvents; i++ {
		e.Push(events.ServiceEvent{
			Message: strconv.Itoa(i),
		})
	}

	suite.Assert().Equal([]events.ServiceEvent(nil), e.Get(0))
	suite.assertEvents([]string{strconv.Itoa(numEvents - 1)}, e.Get(1))

	expected := []string{}
	for i := numEvents - events.MaxEventsToKeep; i < numEvents; i++ {
		expected = append(expected, strconv.Itoa(i))
	}
	suite.assertEvents(expected, e.Get(events.MaxEventsToKeep*10))
	suite.assertEvents(expected[len(expected)-3:], e.Get(3))
}

func TestEventsSuite(t *testing.T) {
	suite.Run(t, new(EventsSuite))
}
