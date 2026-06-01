// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	machinetype "github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// EventsSuite verifies Events API.
type EventsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	nodeCtx context.Context //nolint:containedctx
}

// SuiteName ...
func (suite *EventsSuite) SuiteName() string {
	return "api.EventsSuite"
}

// SetupTest ...
func (suite *EventsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)

	suite.nodeCtx = client.WithNodes(suite.ctx, suite.RandomDiscoveredNodeInternalIP(machinetype.TypeWorker))
}

// TearDownTest ...
func (suite *EventsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestEventsWatch verifies events watch API.
func (suite *EventsSuite) TestEventsWatch() {
	receiveEvents := func(atLeast int, timeout time.Duration, opts ...client.EventsOptionFunc) []client.Event {
		var result []client.Event

		watchCtx, watchCtxCancel := context.WithTimeout(suite.nodeCtx, timeout)
		defer watchCtxCancel()

		suite.Assert().NoError(
			suite.Client.EventsWatch(
				watchCtx, func(ch <-chan client.Event) {
					defer watchCtxCancel()

					for event := range ch {
						result = append(result, event)

						if atLeast > 0 && len(result) >= atLeast {
							return
						}
					}
				}, opts...,
			),
		)

		return result
	}

	allEvents := receiveEvents(21, 5*time.Second, client.WithTailEvents(-1))
	suite.Require().Greater(len(allEvents), 20)

	suite.Assert().Len(receiveEvents(0, 500*time.Millisecond), 0)
	suite.Assert().Len(receiveEvents(5, 5*time.Second, client.WithTailEvents(5)), 5)
	suite.Assert().Len(receiveEvents(20, 5*time.Second, client.WithTailEvents(20)), 20)

	// pick some ID of 15th event in the past; API should return at least 14 events
	// (as check excludes that event with picked ID)
	id := allEvents[len(allEvents)-15].ID
	eventsSinceID := receiveEvents(14, 5*time.Second, client.WithTailID(id))
	suite.Require().GreaterOrEqual(
		len(eventsSinceID),
		14,
	) //  there might some new events since allEvents, but at least 15 should be received
}

func init() {
	allSuites = append(allSuites, new(EventsSuite))
}
