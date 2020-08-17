// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// EventsSuite verifies Events API.
type EventsSuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc

	nodeCtx context.Context
}

// SuiteName ...
func (suite *EventsSuite) SuiteName() string {
	return "api.EventsSuite"
}

// SetupTest ...
func (suite *EventsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)

	suite.nodeCtx = client.WithNodes(suite.ctx, suite.RandomDiscoveredNode())
}

// TearDownTest ...
func (suite *EventsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestServiceEvents verifies that service restart generates events.
func (suite *EventsSuite) TestServiceEvents() {
	const service = "timed" // any restartable service should work

	svcInfo, err := suite.Client.ServiceInfo(suite.nodeCtx, service)
	suite.Require().NoError(err)

	if len(svcInfo) == 0 { // service is not registered (e.g. docker)
		suite.T().Skip(fmt.Sprintf("skipping test as service %q is not registered", service))
	}

	actionsSeen := make(map[machine.ServiceStateEvent_Action]struct{})

	checkExpectedActions := func() error {
		for _, action := range []machine.ServiceStateEvent_Action{
			machine.ServiceStateEvent_STOPPING,
			machine.ServiceStateEvent_FINISHED,
			machine.ServiceStateEvent_WAITING,
			machine.ServiceStateEvent_PREPARING,
			machine.ServiceStateEvent_RUNNING,
		} {
			if _, ok := actionsSeen[action]; !ok {
				return fmt.Errorf("expected action %s was not seen", action)
			}
		}

		return nil
	}

	go func() {
		suite.Assert().NoError(suite.Client.EventsWatch(suite.nodeCtx, func(ch <-chan client.Event) {
			defer suite.ctxCancel()

			for event := range ch {
				if msg, ok := event.Payload.(*machine.ServiceStateEvent); ok && msg.GetService() == service {
					actionsSeen[msg.GetAction()] = struct{}{}
				}

				if checkExpectedActions() == nil {
					return
				}
			}
		}))
	}()

	// wait for event watcher to start
	time.Sleep(200 * time.Millisecond)

	_, err = suite.Client.ServiceRestart(suite.nodeCtx, service)
	suite.Assert().NoError(err)

	<-suite.ctx.Done()

	suite.Require().NoError(checkExpectedActions())
}

// TestEventsWatch verifies events watch API.
func (suite *EventsSuite) TestEventsWatch() {
	receiveEvents := func(opts ...client.EventsOptionFunc) []client.Event {
		result := []client.Event{}

		watchCtx, watchCtxCancel := context.WithCancel(suite.nodeCtx)
		defer watchCtxCancel()

		suite.Assert().NoError(suite.Client.EventsWatch(watchCtx, func(ch <-chan client.Event) {
			defer watchCtxCancel()

			for {
				select {
				case event, ok := <-ch:
					if !ok {
						return
					}

					result = append(result, event)
				case <-time.After(100 * time.Millisecond):
					return
				}
			}
		}, opts...))

		return result
	}

	allEvents := receiveEvents(client.WithTailEvents(-1))
	suite.Require().Greater(len(allEvents), 20)

	suite.Assert().Len(receiveEvents(), 0)
	suite.Assert().Len(receiveEvents(client.WithTailEvents(5)), 5)
	suite.Assert().Len(receiveEvents(client.WithTailEvents(20)), 20)

	// pick some ID of 15th event in the past; API should return at least 14 events
	// (as check excludes that event with picked ID)
	id := allEvents[len(allEvents)-15].ID
	eventsSinceID := receiveEvents(client.WithTailID(id))
	suite.Require().GreaterOrEqual(len(eventsSinceID), 14) //  there might some new events since allEvents, but at least 15 should be received
}

func init() {
	allSuites = append(allSuites, new(EventsSuite))
}
