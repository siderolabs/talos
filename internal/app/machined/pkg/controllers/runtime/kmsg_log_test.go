// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/go-retry/retry"
	"github.com/siderolabs/siderolink/pkg/logreceiver"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	talosruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type logHandler struct {
	mu    sync.Mutex
	count int
}

// HandleLog implements logreceiver.Handler.
func (s *logHandler) HandleLog(srcAddr netip.Addr, msg map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count++
}

func (s *logHandler) getCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count
}

type KmsgLogDeliverySuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	drainer *talosruntime.Drainer
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	handler1, handler2 *logHandler

	listener1, listener2 net.Listener
	srv1, srv2           *logreceiver.Server
}

func (suite *KmsgLogDeliverySuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Second)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	logger := zaptest.NewLogger(suite.T())

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)

	suite.handler1 = &logHandler{}
	suite.handler2 = &logHandler{}

	suite.listener1, err = net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.listener2, err = net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.srv1 = logreceiver.NewServer(logger, suite.listener1, suite.handler1.HandleLog)

	suite.srv2 = logreceiver.NewServer(logger, suite.listener2, suite.handler2.HandleLog)

	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.srv1.Serve() //nolint:errcheck
	}()

	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.srv2.Serve() //nolint:errcheck
	}()

	suite.drainer = talosruntime.NewDrainer()

	suite.Require().NoError(
		suite.runtime.RegisterController(
			&runtimectrl.KmsgLogDeliveryController{
				Drainer: suite.drainer,
			},
		),
	)

	status := network.NewStatus(network.NamespaceName, network.StatusID)
	status.TypedSpec().AddressReady = true

	suite.Require().NoError(suite.state.Create(suite.ctx, status))
}

func (suite *KmsgLogDeliverySuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *KmsgLogDeliverySuite) TestDeliverySingleDestination() {
	suite.startRuntime()

	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, kmsgLogConfig))

	// controller should deliver some kernel logs from host's kmsg buffer
	suite.assertLogsSeen(suite.handler1)
}

func (suite *KmsgLogDeliverySuite) TestDeliveryMultipleDestinations() {
	suite.startRuntime()

	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
		{
			Scheme: "tcp",
			Host:   suite.listener2.Addr().String(),
		},
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, kmsgLogConfig))

	// controller should deliver logs to both destinations
	suite.assertLogsSeen(suite.handler1)
	suite.assertLogsSeen(suite.handler2)
}

func (suite *KmsgLogDeliverySuite) TestDeliveryOneDeadDestination() {
	suite.startRuntime()

	// stop one listener
	suite.Require().NoError(suite.listener1.Close())

	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
		{
			Scheme: "tcp",
			Host:   suite.listener2.Addr().String(),
		},
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, kmsgLogConfig))

	// controller should deliver logs to live destination
	suite.assertLogsSeen(suite.handler2)
}

func (suite *KmsgLogDeliverySuite) TestDeliveryAllDeadDestinations() {
	suite.startRuntime()

	// stop all listeners
	suite.Require().NoError(suite.listener1.Close())
	suite.Require().NoError(suite.listener2.Close())

	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
		{
			Scheme: "tcp",
			Host:   suite.listener2.Addr().String(),
		},
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, kmsgLogConfig))
}

func (suite *KmsgLogDeliverySuite) TestDrain() {
	suite.startRuntime()

	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
	}

	suite.Require().NoError(suite.state.Create(suite.ctx, kmsgLogConfig))

	// wait for controller to start delivering some logs
	suite.assertLogsSeen(suite.handler1)

	// drain should be successful, i.e. controller should stop on its own before context is canceled
	suite.Assert().NoError(suite.drainer.Drain(suite.ctx))
}

func (suite *KmsgLogDeliverySuite) assertLogsSeen(handler *logHandler) {
	err := retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(
		func() error {
			if handler.getCount() == 0 {
				return retry.ExpectedErrorf("no logs received")
			}

			return nil
		},
	)
	suite.Require().NoError(err)
}

func (suite *KmsgLogDeliverySuite) TearDownTest() {
	suite.srv1.Stop()
	suite.srv2.Stop()

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKmsgLogDeliverySuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	suite.Run(t, new(KmsgLogDeliverySuite))
}
