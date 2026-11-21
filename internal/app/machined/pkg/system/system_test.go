// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"errors"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
)

var (
	onceRuntime    sync.Once
	runtimeRuntime runtime.Runtime
)

func newRuntime(t *testing.T) runtime.Runtime {
	onceRuntime.Do(func() {
		e := v1alpha1.NewEvents(1000, 10)

		t.Setenv("PLATFORM", "container")

		s, err := v1alpha1.NewState()
		require.NoError(t, err)

		l := logging.NewCircularBufferLoggingManager(log.New(t.Output(), "fallback logger: ", log.Flags()))

		runtimeRuntime = v1alpha1.NewRuntime(s, e, l)
	})

	return runtimeRuntime
}

type SystemServicesSuite struct {
	suite.Suite
}

func (suite *SystemServicesSuite) TestStartShutdown() {
	system.Services(newRuntime(suite.T())).LoadAndStart(
		&MockService{name: "containerd"},
		&MockService{name: "trustd", dependencies: []string{"containerd"}},
		&MockService{name: "machined", dependencies: []string{"containerd", "trustd"}},
	)
	time.Sleep(10 * time.Millisecond)

	suite.Require().NoError(system.Services(nil).Unload(context.Background(), "trustd", "notrunning"))
}

func (suite *SystemServicesSuite) TestStartStop() {
	system.Services(newRuntime(suite.T())).LoadAndStart(
		&MockService{name: "yolo"},
	)

	time.Sleep(10 * time.Millisecond)

	err := system.Services(nil).Stop(
		context.TODO(), "yolo",
	)
	suite.Assert().NoError(err)
}

func (suite *SystemServicesSuite) TestStopWithRevDeps() {
	system.Services(newRuntime(suite.T())).LoadAndStart(
		&MockService{name: "cri"},
		&MockService{name: "networkd", dependencies: []string{"cri"}},
		&MockService{name: "vland", dependencies: []string{"networkd"}},
	)
	time.Sleep(10 * time.Millisecond)

	// stopping cri should stop all services
	suite.Require().NoError(system.Services(newRuntime(suite.T())).StopWithRevDepenencies(context.Background(), "cri"))

	// no services should be running
	for _, name := range []string{"cri", "networkd", "vland"} {
		svc, running, err := system.Services(nil).IsRunning(name)
		suite.Require().NoError(err)
		suite.Assert().NotNil(svc)
		suite.Assert().False(running)
	}

	system.Services(nil).Shutdown(context.TODO())
}

func serviceState(id string) string {
	for _, svcRunner := range system.Services(nil).List() {
		if proto := svcRunner.AsProto(); proto.Id == id {
			return proto.State
		}
	}

	return ""
}

// TestStopDuringCondition verifies that stopping a service while it is still
// waiting on its start condition is treated as a clean stop, and doesn't leave
// the service stuck in the Failed state (regression test).
func (suite *SystemServicesSuite) TestStopDuringCondition() {
	cond := NewMockCondition("test condition")

	system.Services(newRuntime(suite.T())).LoadAndStart(
		&MockService{name: "stopcond", condition: cond},
	)

	// wait until the service is waiting on the condition
	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		if state := serviceState("stopcond"); state != events.StateWaiting.String() {
			return retry.ExpectedErrorf("service should be waiting, got %q", state)
		}

		return nil
	}))

	// stopping the service while it waits on the condition should be a clean
	// stop, not a failure (it used to stick in StateFailed)
	suite.Require().NoError(system.Services(nil).Stop(context.Background(), "stopcond"))

	suite.Assert().Equal(events.StateFinished.String(), serviceState("stopcond"))
}

// TestStartAfterStopFailed verifies that stopping a service which has already failed
// doesn't stop the service when it is started the next time (regression test).
func (suite *SystemServicesSuite) TestStartAfterStopFailed() {
	svc := &MockService{name: "stopfailed", preError: errors.New("pre failed")}

	system.Services(newRuntime(suite.T())).LoadAndStart(svc)

	waitForState := func(expected events.ServiceState) {
		suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
			if state := serviceState("stopfailed"); state != expected.String() {
				return retry.ExpectedErrorf("service should be %s, got %q", expected, state)
			}

			return nil
		}))
	}

	waitForState(events.StateFailed)

	// the service is not running, so there is nothing to stop
	suite.Require().NoError(system.Services(nil).Stop(context.Background(), "stopfailed"))

	state, err := system.Services(nil).GetState("stopfailed")
	suite.Require().NoError(err)
	suite.Assert().Equal(events.StateFailed, state)

	// the failure is fixed, and the service is started again
	svc.preError = nil

	suite.Require().NoError(system.Services(nil).Start("stopfailed"))

	waitForState(events.StateRunning)

	// the service keeps running
	time.Sleep(100 * time.Millisecond)

	suite.Assert().Equal(events.StateRunning.String(), serviceState("stopfailed"))

	suite.Require().NoError(system.Services(nil).Stop(context.Background(), "stopfailed"))
}

func TestSystemServicesSuite(t *testing.T) {
	suite.Run(t, new(SystemServicesSuite))
}
