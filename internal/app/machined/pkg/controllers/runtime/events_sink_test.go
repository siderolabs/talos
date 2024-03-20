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

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	eventsapi "github.com/siderolabs/siderolink/api/events"
	"github.com/siderolabs/siderolink/pkg/events"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

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

type EventsSinkSuite struct {
	suite.Suite

	events  *v1alpha1.Events
	state   state.State
	handler *handler
	server  *grpc.Server
	sink    *events.Sink

	runtime *runtime.Runtime
	drainer *talosruntime.Drainer
	wg      sync.WaitGroup
	eg      errgroup.Group

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *EventsSinkSuite) SetupTest() {
	suite.events = v1alpha1.NewEvents(1000, 10)

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.handler = &handler{}
	suite.drainer = talosruntime.NewDrainer()

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&controllerruntime.EventsSinkController{
				V1Alpha1Events: suite.events,
				Drainer:        suite.drainer,
			},
		),
	)

	status := network.NewStatus(network.NamespaceName, network.StatusID)
	status.TypedSpec().AddressReady = true

	suite.Require().NoError(suite.state.Create(suite.ctx, status))

	suite.startRuntime()
}

func (suite *EventsSinkSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *EventsSinkSuite) startServer(ctx context.Context) string {
	suite.sink = events.NewSink(
		suite.handler,
		[]proto.Message{
			&machine.AddressEvent{},
			&machine.PhaseEvent{},
		})

	lis, err := net.Listen("tcp", "localhost:0")
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

func (suite *EventsSinkSuite) TestPublish() {
	ctx, cancel := context.WithCancel(suite.ctx)
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

	suite.Require().Equal(0, len(suite.handler.events))

	endpoint := suite.startServer(ctx)
	config := runtimeres.NewEventSinkConfig()
	config.TypedSpec().Endpoint = endpoint
	suite.Require().NoError(suite.state.Create(ctx, config))

	suite.Require().NoError(retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(
		func() error {
			suite.handler.eventsMu.Lock()
			defer suite.handler.eventsMu.Unlock()

			if len(suite.handler.events) != 2 {
				return retry.ExpectedErrorf("expected 2 events, got %d", len(suite.handler.events))
			}

			return nil
		},
	))

	suite.events.Publish(
		ctx,
		&machine.PhaseEvent{
			Phase:  "test",
			Action: machine.PhaseEvent_STOP,
		},
	)

	suite.Require().NoError(retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(
		func() error {
			suite.handler.eventsMu.Lock()
			defer suite.handler.eventsMu.Unlock()

			if len(suite.handler.events) != 3 {
				return retry.ExpectedErrorf("expected 3 events, got %d", len(suite.handler.events))
			}

			return nil
		},
	))
}

func (suite *EventsSinkSuite) TestDrain() {
	ctx, cancel := context.WithCancel(suite.ctx)
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

	suite.Require().Equal(0, len(suite.handler.events))

	// first, publish wrong endpoint
	badLis, err := net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	badEndpoint := badLis.Addr().String()
	suite.Require().NoError(badLis.Close())

	config := runtimeres.NewEventSinkConfig()
	config.TypedSpec().Endpoint = badEndpoint
	suite.Require().NoError(suite.state.Create(ctx, config))

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

			_, updateErr := safe.StateUpdateWithConflicts(
				ctx, suite.state, runtimeres.NewEventSinkConfig().Metadata(),
				func(cfg *runtimeres.EventSinkConfig) error {
					cfg.TypedSpec().Endpoint = endpoint

					return nil
				})

			return updateErr
		},
	)

	suite.Require().NoError(retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(
		func() error {
			suite.handler.eventsMu.Lock()
			defer suite.handler.eventsMu.Unlock()

			if len(suite.handler.events) != 20 {
				return retry.ExpectedErrorf("expected 20 events, got %d", len(suite.handler.events))
			}

			return nil
		},
	))

	suite.Require().NoError(eg.Wait())
}

func (suite *EventsSinkSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.Require().NoError(suite.eg.Wait())

	suite.wg.Wait()
}

func TestEventsSinkSuite(t *testing.T) {
	suite.Run(t, new(EventsSinkSuite))
}
