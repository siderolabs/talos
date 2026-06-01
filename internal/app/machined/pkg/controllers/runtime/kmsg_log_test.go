// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"net"
	"net/netip"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/siderolink/pkg/logreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
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
func (s *logHandler) HandleLog(srcAddr netip.Addr, msg map[string]any) {
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
	ctest.DefaultSuite

	drainer *talosruntime.Drainer

	handler1, handler2 *logHandler

	listener1, listener2 net.Listener
	srv1, srv2           *logreceiver.Server

	serversWG sync.WaitGroup
}

func (suite *KmsgLogDeliverySuite) TestDeliverySingleDestination() {
	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
	}

	suite.Create(kmsgLogConfig)

	// controller should deliver some kernel logs from host's kmsg buffer
	suite.assertLogsSeen(suite.handler1)
}

func (suite *KmsgLogDeliverySuite) TestDeliveryMultipleDestinations() {
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

	suite.Create(kmsgLogConfig)

	// controller should deliver logs to both destinations
	suite.assertLogsSeen(suite.handler1)
	suite.assertLogsSeen(suite.handler2)
}

func (suite *KmsgLogDeliverySuite) TestDeliveryOneDeadDestination() {
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

	suite.Create(kmsgLogConfig)

	// controller should deliver logs to live destination
	suite.assertLogsSeen(suite.handler2)
}

func (suite *KmsgLogDeliverySuite) TestDeliveryAllDeadDestinations() {
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

	suite.Create(kmsgLogConfig)
}

func (suite *KmsgLogDeliverySuite) TestDrain() {
	kmsgLogConfig := runtimeres.NewKmsgLogConfig()
	kmsgLogConfig.TypedSpec().Destinations = []*url.URL{
		{
			Scheme: "tcp",
			Host:   suite.listener1.Addr().String(),
		},
	}

	suite.Create(kmsgLogConfig)

	// wait for controller to start delivering some logs
	suite.assertLogsSeen(suite.handler1)

	// drain should be successful, i.e. controller should stop on its own before context is canceled
	suite.Assert().NoError(suite.drainer.Drain(suite.Ctx()))
}

func (suite *KmsgLogDeliverySuite) assertLogsSeen(handler *logHandler) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		assert.NotZero(collect, handler.getCount(), "no logs received")
	}, 5*time.Second, 100*time.Millisecond)
}

func TestKmsgLogDeliverySuite(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	kmsgSuite := &KmsgLogDeliverySuite{}
	kmsgSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			logger := zaptest.NewLogger(s.T())

			kmsgSuite.handler1 = &logHandler{}
			kmsgSuite.handler2 = &logHandler{}

			var err error

			kmsgSuite.listener1, err = (&net.ListenConfig{}).Listen(s.Ctx(), "tcp", "localhost:0")
			s.Require().NoError(err)

			kmsgSuite.listener2, err = (&net.ListenConfig{}).Listen(s.Ctx(), "tcp", "localhost:0")
			s.Require().NoError(err)

			kmsgSuite.srv1 = logreceiver.NewServer(logger, kmsgSuite.listener1, kmsgSuite.handler1.HandleLog)
			kmsgSuite.srv2 = logreceiver.NewServer(logger, kmsgSuite.listener2, kmsgSuite.handler2.HandleLog)

			kmsgSuite.serversWG.Go(func() {
				kmsgSuite.srv1.Serve() //nolint:errcheck
			})

			kmsgSuite.serversWG.Go(func() {
				kmsgSuite.srv2.Serve() //nolint:errcheck
			})

			kmsgSuite.drainer = talosruntime.NewDrainer()

			s.Require().NoError(s.Runtime().RegisterController(&runtimectrl.KmsgLogDeliveryController{
				Drainer: kmsgSuite.drainer,
			}))

			status := network.NewStatus(network.NamespaceName, network.StatusID)
			status.TypedSpec().AddressReady = true

			s.Require().NoError(s.State().Create(s.Ctx(), status))
		},
		AfterTearDown: func(*ctest.DefaultSuite) {
			kmsgSuite.srv1.Stop()
			kmsgSuite.srv2.Stop()

			kmsgSuite.serversWG.Wait()
		},
	}

	suite.Run(t, kmsgSuite)
}
