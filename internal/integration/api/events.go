// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/client"
)

// EventsSuite verifies Events API.
type EventsSuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EventsSuite) SuiteName() string {
	return "api.EventsSuite"
}

// SetupTest ...
func (suite *EventsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *EventsSuite) TearDownTest() {
	suite.ctxCancel()
}

// TestServiceEvents verifies that service restart generates events.
func (suite *EventsSuite) TestServiceEvents() {
	const service = "timed" // any restartable service should work

	ctx, ctxCancel := context.WithTimeout(suite.ctx, 30*time.Second)
	defer ctxCancel()

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
		suite.Assert().NoError(suite.Client.EventsWatch(ctx, func(ch <-chan client.Event) {
			defer ctxCancel()

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

	_, err := suite.Client.ServiceRestart(ctx, service)
	suite.Assert().NoError(err)

	<-ctx.Done()

	suite.Require().NoError(checkExpectedActions())
}

func init() {
	allSuites = append(allSuites, new(EventsSuite))
}
