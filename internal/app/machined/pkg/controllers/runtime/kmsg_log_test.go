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
	"github.com/talos-systems/siderolink/pkg/logreceiver"
	"inet.af/netaddr"

	controllerruntime "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	talosruntime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type logHandler struct {
	mu    sync.Mutex
	count int
}

// HandleLog implements logreceiver.Handler.
func (s *logHandler) HandleLog(srcAddr netaddr.IP, msg map[string]interface{}) {
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

	state   state.State
	cmdline *procfs.Cmdline

	runtime *runtime.Runtime
	drainer *talosruntime.Drainer
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

	handler *logHandler

	srv *logreceiver.Server
}

func (suite *KmsgLogDeliverySuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	logger := logging.Wrap(log.Writer())

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logger)
	suite.Require().NoError(err)

	suite.handler = &logHandler{}

	listener, err := net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.srv, err = logreceiver.NewServer(logger, listener, suite.handler.HandleLog)
	suite.Require().NoError(err)

	go func() {
		suite.srv.Serve() //nolint:errcheck
	}()

	suite.cmdline = procfs.NewCmdline(fmt.Sprintf("%s=%s", constants.KernelParamLoggingKernel, fmt.Sprintf("tcp://%s", listener.Addr())))
	suite.drainer = talosruntime.NewDrainer()

	suite.Require().NoError(suite.runtime.RegisterController(&controllerruntime.KmsgLogDeliveryController{
		Cmdline: suite.cmdline,
		Drainer: suite.drainer,
	}))

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

func (suite *KmsgLogDeliverySuite) TestDelivery() {
	suite.startRuntime()

	// controller should deliver some kernel logs from host's kmsg buffer
	err := retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(func() error {
		if suite.handler.getCount() == 0 {
			return retry.ExpectedErrorf("no logs received")
		}

		return nil
	})
	suite.Require().NoError(err)
}

func (suite *KmsgLogDeliverySuite) TestDrain() {
	suite.startRuntime()

	// wait for controller to start delivering some logs
	err := retry.Constant(time.Second*5, retry.WithUnits(time.Millisecond*100)).Retry(func() error {
		if suite.handler.getCount() == 0 {
			return retry.ExpectedErrorf("no logs received")
		}

		return nil
	})
	suite.Require().NoError(err)

	// drain should be successful, i.e. controller should stop on its own before context is canceled
	suite.Assert().NoError(suite.drainer.Drain(suite.ctx))
}

func (suite *KmsgLogDeliverySuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.srv.Stop()

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestKmsgLogDeliverySuite(t *testing.T) {
	suite.Run(t, new(KmsgLogDeliverySuite))
}
