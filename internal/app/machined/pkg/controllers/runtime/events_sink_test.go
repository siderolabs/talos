// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	eventsapi "github.com/talos-systems/siderolink/api/events"
	"github.com/talos-systems/siderolink/pkg/events"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	controllerruntime "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	talosruntime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type handler struct {
	eventsMu sync.Mutex
	events   []events.Event
}

// HandleEvent implements events.Adapter.
func (s *handler) HandleEvent(e events.Event) error {
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
	cmdline *procfs.Cmdline
	server  *grpc.Server
	sink    *events.Sink

	runtime *runtime.Runtime
	drainer *talosruntime.Drainer
	wg      sync.WaitGroup
	eg      errgroup.Group

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *EventsSinkSuite) SetupTest() {
	suite.events = v1alpha1.NewEvents(1000, 10)

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.handler = &handler{}
	suite.cmdline = procfs.NewCmdline(fmt.Sprintf("%s=%s", constants.KernelParamEventsSink, "localhost"))
	suite.drainer = talosruntime.NewDrainer()

	suite.Require().NoError(suite.runtime.RegisterController(&controllerruntime.EventsSinkController{
		V1Alpha1Events: suite.events,
		Cmdline:        suite.cmdline,
		Drainer:        suite.drainer,
	}))

	suite.startRuntime()
}

func (suite *EventsSinkSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *EventsSinkSuite) startServer(ctx context.Context) {
	suite.sink = events.NewSink(suite.handler)

	status := network.NewStatus(network.NamespaceName, network.StatusID)
	status.TypedSpec().AddressReady = true

	suite.Require().NoError(suite.state.Create(ctx, status))

	lis, err := net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	param := procfs.NewParameter(constants.KernelParamEventsSink)
	param.Append(lis.Addr().String())

	suite.cmdline.Set(constants.KernelParamEventsSink, param)
	suite.server = grpc.NewServer()
	eventsapi.RegisterEventSinkServiceServer(suite.server, suite.sink)

	suite.eg.Go(func() error {
		<-ctx.Done()

		suite.server.Stop()

		return nil
	})

	suite.eg.Go(func() error {
		return suite.server.Serve(lis)
	})
}

func (suite *EventsSinkSuite) TestPublish() {
	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()

	suite.events.Publish(&machine.AddressEvent{
		Hostname: "localhost",
	})

	suite.events.Publish(&machine.PhaseEvent{
		Phase:  "test",
		Action: machine.PhaseEvent_START,
	})

	suite.Require().Equal(0, len(suite.handler.events))

	suite.startServer(ctx)

	err := retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(func() error {
		suite.handler.eventsMu.Lock()
		defer suite.handler.eventsMu.Unlock()

		if len(suite.handler.events) != 2 {
			return retry.ExpectedErrorf("expected 2 events")
		}

		return nil
	})
	suite.Require().NoError(err)
}

func (suite *EventsSinkSuite) TestDrain() {
	ctx, cancel := context.WithCancel(suite.ctx)
	defer cancel()

	for i := 0; i < 10; i++ {
		suite.events.Publish(&machine.PhaseEvent{
			Phase:  "test",
			Action: machine.PhaseEvent_START,
		})
		suite.events.Publish(&machine.PhaseEvent{
			Phase:  "test",
			Action: machine.PhaseEvent_STOP,
		})
	}

	suite.Require().Equal(0, len(suite.handler.events))

	time.Sleep(time.Second * 1)

	c, abort := context.WithTimeout(context.Background(), time.Second*5)
	defer abort()

	var eg errgroup.Group

	eg.Go(func() error {
		return suite.drainer.Drain(c)
	})

	eg.Go(func() error {
		time.Sleep(time.Millisecond * 300)

		suite.startServer(ctx)

		return nil
	})

	err := retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(func() error {
		suite.handler.eventsMu.Lock()
		defer suite.handler.eventsMu.Unlock()

		if len(suite.handler.events) != 20 {
			return retry.ExpectedErrorf("expected 20 events, got %d", len(suite.handler.events))
		}

		return nil
	})
	suite.Require().NoError(err)

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
