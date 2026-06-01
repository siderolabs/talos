// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-retry/retry"
	eventsapi "github.com/siderolabs/siderolink/api/events"
	"github.com/siderolabs/siderolink/pkg/events"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	controllerruntime "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	talosruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type handler struct {
	eventsMu sync.Mutex
	events   []events.Event
}

// HandleEvent implements events.Adapter.
func (s *handler) HandleEvent(ctx context.Context, e events.Event) error {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	s.events = append(s.events, e)

	return nil
}

func (s *handler) len() int {
	s.eventsMu.Lock()
	defer s.eventsMu.Unlock()

	return len(s.events)
}

type EventsSinkSuite struct {
	ctest.DefaultSuite

	events  *v1alpha1.Events
	handler *handler
	server  *grpc.Server
	sink    *events.Sink

	drainer *talosruntime.Drainer
	eg      errgroup.Group
}

func (suite *EventsSinkSuite) startServer(ctx context.Context) string {
	suite.sink = events.NewSink(
		suite.handler,
		[]proto.Message{
			&machine.AddressEvent{},
			&machine.PhaseEvent{},
		},
	)

	lis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.server = grpc.NewServer()
	eventsapi.RegisterEventSinkServiceServer(suite.server, suite.sink)

	suite.eg.Go(
		func() error {
			<-ctx.Done()

			suite.server.Stop()

			return nil
		},
	)

	suite.eg.Go(
		func() error {
			return suite.server.Serve(lis)
		},
	)

	return lis.Addr().String()
}

func (suite *EventsSinkSuite) assertEventCount(expected int) {
	suite.Require().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			if got := suite.handler.len(); got != expected {
				return retry.ExpectedErrorf("expected %d events, got %d", expected, got)
			}

			return nil
		},
	))
}

func (suite *EventsSinkSuite) TestPublish() {
	ctx, cancel := context.WithCancel(suite.Ctx())
	defer cancel()

	suite.events.Publish(
		ctx,
		&machine.AddressEvent{
			Hostname: "localhost",
		},
	)

	suite.events.Publish(
		ctx,
		&machine.PhaseEvent{
			Phase:  "test",
			Action: machine.PhaseEvent_START,
		},
	)

	suite.Require().Equal(0, suite.handler.len())

	endpoint := suite.startServer(ctx)
	config := runtimeres.NewEventSinkConfig()
	config.TypedSpec().Endpoint = endpoint
	suite.Create(config)

	suite.assertEventCount(2)

	suite.events.Publish(
		ctx,
		&machine.PhaseEvent{
			Phase:  "test",
			Action: machine.PhaseEvent_STOP,
		},
	)

	suite.assertEventCount(3)
}

func (suite *EventsSinkSuite) TestDrain() {
	ctx, cancel := context.WithCancel(suite.Ctx())
	defer cancel()

	for range 10 {
		suite.events.Publish(
			ctx,
			&machine.PhaseEvent{
				Phase:  "test",
				Action: machine.PhaseEvent_START,
			},
		)
		suite.events.Publish(
			ctx,
			&machine.PhaseEvent{
				Phase:  "test",
				Action: machine.PhaseEvent_STOP,
			},
		)
	}

	suite.Require().Equal(0, suite.handler.len())

	// first, publish wrong endpoint
	badLis, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "localhost:0")
	suite.Require().NoError(err)

	badEndpoint := badLis.Addr().String()
	suite.Require().NoError(badLis.Close())

	config := runtimeres.NewEventSinkConfig()
	config.TypedSpec().Endpoint = badEndpoint
	suite.Create(config)

	suite.T().Logf("%s starting bad server at %s", time.Now().Format(time.RFC3339), badEndpoint)

	time.Sleep(time.Second * 1)

	drainCtx, drainCtxCancel := context.WithTimeout(ctx, time.Second*5)
	defer drainCtxCancel()

	var eg errgroup.Group

	eg.Go(
		func() error {
			suite.T().Logf("%s starting drain", time.Now().Format(time.RFC3339))

			return suite.drainer.Drain(drainCtx)
		},
	)

	eg.Go(
		func() error {
			// start real server with delay
			time.Sleep(300 * time.Millisecond)

			endpoint := suite.startServer(ctx)

			suite.T().Logf("%s starting real server at %s", time.Now().Format(time.RFC3339), endpoint)

			_, err := safe.StateUpdateWithConflicts(suite.Ctx(), suite.State(), config.Metadata(), func(cfg *runtimeres.EventSinkConfig) error {
				cfg.TypedSpec().Endpoint = endpoint

				return nil
			})

			return err
		},
	)

	suite.assertEventCount(20)

	suite.Require().NoError(eg.Wait())
}

func TestEventsSinkSuite(t *testing.T) {
	sinkSuite := &EventsSinkSuite{}
	sinkSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 30 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			sinkSuite.events = v1alpha1.NewEvents(1000, 10)
			sinkSuite.handler = &handler{}
			sinkSuite.drainer = talosruntime.NewDrainer()

			s.Require().NoError(s.Runtime().RegisterController(&controllerruntime.EventsSinkController{
				V1Alpha1Events: sinkSuite.events,
				Drainer:        sinkSuite.drainer,
			}))

			status := network.NewStatus(network.NamespaceName, network.StatusID)
			status.TypedSpec().AddressReady = true

			s.Require().NoError(s.State().Create(s.Ctx(), status))
		},
		AfterTearDown: func(*ctest.DefaultSuite) {
			sinkSuite.Require().NoError(sinkSuite.eg.Wait())
		},
	}

	suite.Run(t, sinkSuite)
}
