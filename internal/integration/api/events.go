// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/client"
	machinetype "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
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

	suite.nodeCtx = client.WithNodes(suite.ctx, suite.RandomDiscoveredNode(machinetype.TypeJoin))
}

// TearDownTest ...
func (suite *EventsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
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
