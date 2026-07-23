// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/pid"
	"github.com/siderolabs/talos/pkg/conditions"
)

type ServiceRunnerSuite struct {
	suite.Suite
}

func (suite *ServiceRunnerSuite) assertStateSequence(expectedStates []events.ServiceState, sr *system.ServiceRunner) {
	states := make([]events.ServiceState, 0, 1000)

	for _, event := range sr.GetEventHistory(1000) {
		states = append(states, event.State)
	}

	suite.Assert().Equal(expectedStates, states)
}

func (suite *ServiceRunnerSuite) TestFullFlow() {
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), &MockService{
		condition: conditions.None(),
	}, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		state := sr.AsProto().State
		if state != events.StateRunning.String() {
			return retry.ExpectedErrorf("service should be running")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StateWaiting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
	}, sr)

	protoService := sr.AsProto()
	suite.Assert().Equal("MockRunner", protoService.Id)
	suite.Assert().Equal("Running", protoService.State)
	suite.Assert().True(protoService.Health.Unknown)
	suite.Assert().Len(protoService.Events.Events, 5)
}

func (suite *ServiceRunnerSuite) TestFullFlowHealthy() {
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), &MockHealthcheckedService{}, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		health := sr.AsProto().Health
		if health.Unknown || !health.Healthy {
			return retry.ExpectedErrorf("service should be healthy")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
		events.StateRunning, // one more notification when service is healthy
	}, sr)
}

func TestServiceRunnerPublishesHealthReadyBeforeRunning(t *testing.T) {
	healthCheckStarted := make(chan struct{})
	healthStateUpdated := make(chan struct{})
	allowHealthCheck := make(chan struct{})
	allowRunning := make(chan struct{})
	runningPublished := make(chan struct{})

	runnr := &blockedRunningRunner{
		allowRunning:     allowRunning,
		runningPublished: runningPublished,
		exitCh:           make(chan error),
	}

	svc := &blockedHealthcheckedService{
		MockService: MockService{
			runner: runnr,
		},
		healthCheckStarted: healthCheckStarted,
		healthStateUpdated: healthStateUpdated,
		allowHealthCheck:   allowHealthCheck,
	}

	sr := system.NewServiceRunner(system.Services(newRuntime(t)), svc, newRuntime(t))
	errCh := make(chan error, 1)

	go func() {
		errCh <- sr.Run()
	}()

	var (
		releaseHealthCheck sync.Once
		releaseRunning     sync.Once
	)

	t.Cleanup(func() {
		releaseHealthCheck.Do(func() { close(allowHealthCheck) })
		releaseRunning.Do(func() { close(allowRunning) })

		sr.Shutdown()
		require.NoError(t, <-errCh)
	})

	select {
	case <-healthCheckStarted:
	case <-time.After(time.Minute):
		require.FailNow(t, "health check did not start")
	}

	releaseHealthCheck.Do(func() { close(allowHealthCheck) })

	// The next check starts only after health.Run publishes the first result.
	select {
	case <-healthStateUpdated:
	case <-time.After(time.Minute):
		require.FailNow(t, "health state was not updated")
	}

	releaseRunning.Do(func() { close(allowRunning) })

	select {
	case <-runningPublished:
	case <-time.After(time.Minute):
		require.FailNow(t, "running state was not published")
	}

	findLastEvent := func(state events.ServiceState) *events.ServiceEvent {
		history := sr.GetEventHistory(1000)

		for i := len(history) - 1; i >= 0; i-- {
			if history[i].State == state {
				return &history[i]
			}
		}

		return nil
	}

	runningEvent := findLastEvent(events.StateRunning)

	require.NotNil(t, runningEvent)
	require.False(t, runningEvent.Health.AsProto().Unknown)
	require.True(t, runningEvent.Health.AsProto().Healthy)

	sr.UpdateState(t.Context(), events.StateStopping, "Stopping")

	stoppingEvent := findLastEvent(events.StateStopping)

	require.NotNil(t, stoppingEvent)
	require.True(t, stoppingEvent.Health.AsProto().Unknown)
}

type blockedHealthcheckedService struct {
	MockService

	healthCheckStarted chan<- struct{}
	healthStateUpdated chan<- struct{}
	allowHealthCheck   <-chan struct{}
	healthCheckCount   atomic.Int32
}

func (svc *blockedHealthcheckedService) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		switch svc.healthCheckCount.Add(1) {
		case 1:
			close(svc.healthCheckStarted)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-svc.allowHealthCheck:
				return nil
			}
		case 2:
			// Block further updates after proving the first result was published.
			close(svc.healthStateUpdated)

			<-ctx.Done()

			return ctx.Err()
		default:
			return nil
		}
	}
}

func (svc *blockedHealthcheckedService) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.Settings{
		Timeout: time.Minute,
		Period:  time.Millisecond,
	}
}

type blockedRunningRunner struct {
	allowRunning     <-chan struct{}
	runningPublished chan<- struct{}
	exitCh           chan error
	stopOnce         sync.Once
}

func (runnr *blockedRunningRunner) Open() error {
	return nil
}

func (runnr *blockedRunningRunner) Close() error {
	return nil
}

func (runnr *blockedRunningRunner) Run(eventSink events.Recorder, _ pid.Recorder) error {
	<-runnr.allowRunning

	eventSink(events.StateRunning, "Running")
	close(runnr.runningPublished)

	return <-runnr.exitCh
}

func (runnr *blockedRunningRunner) Stop() error {
	runnr.stopOnce.Do(func() { close(runnr.exitCh) })

	return nil
}

func (runnr *blockedRunningRunner) String() string {
	return "blockedRunningRunner()"
}

func (suite *ServiceRunnerSuite) TestFullFlowHealthChanges() {
	m := MockHealthcheckedService{
		MockService: MockService{
			condition: conditions.None(),
		},
	}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), &m, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		health := sr.AsProto().Health
		if health.Unknown || !health.Healthy {
			return retry.ExpectedErrorf("service should be healthy")
		}

		return nil
	}))

	m.SetHealthy(false)

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		health := sr.AsProto().Health
		if health.Unknown || health.Healthy {
			return retry.ExpectedErrorf("service should be not healthy")
		}

		return nil
	}))

	m.SetHealthy(true)

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		health := sr.AsProto().Health
		if health.Unknown || !health.Healthy {
			return retry.ExpectedErrorf("service should be healthy")
		}

		return nil
	}))

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StateWaiting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
		events.StateRunning, // initial: healthy
		events.StateRunning, // not healthy
		events.StateRunning, // once again healthy
	}, sr)
}

func (suite *ServiceRunnerSuite) TestWaitingDescriptionChange() {
	oldWaitConditionCheckInterval := system.WaitConditionCheckInterval
	system.WaitConditionCheckInterval = 10 * time.Millisecond

	defer func() {
		system.WaitConditionCheckInterval = oldWaitConditionCheckInterval
	}()

	cond1 := NewMockCondition("cond1")
	cond2 := NewMockCondition("cond2")
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), &MockService{
		condition: conditions.WaitForAll(cond1, cond2),
	}, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		state := sr.AsProto().State
		if state != events.StateWaiting.String() {
			return retry.ExpectedErrorf("service should be waiting")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	close(cond1.done)

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		events := sr.AsProto().Events.Events

		lastMsg := events[len(events)-1].Msg
		if lastMsg != "Waiting for cond2" {
			return retry.ExpectedErrorf("service should be waiting on 2nd condition")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	close(cond2.done)

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		state := sr.AsProto().State
		if state != events.StateRunning.String() {
			return retry.ExpectedErrorf("service should be running")
		}

		return nil
	}))

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StateWaiting,
		events.StateWaiting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
	}, sr)

	events := sr.GetEventHistory(10000)
	suite.Assert().Equal("Waiting for cond1, cond2", events[1].Message)
	suite.Assert().Equal("Waiting for cond2", events[2].Message)
}

func (suite *ServiceRunnerSuite) TestPreStageFail() {
	svc := &MockService{
		preError: errors.New("pre failed"),
	}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), svc, newRuntime(suite.T()))
	err := sr.Run()

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StatePreparing,
	}, sr)
	suite.Assert().EqualError(err, "failed to run pre stage: pre failed")
}

func (suite *ServiceRunnerSuite) TestRunnerStageFail() {
	svc := &MockService{
		runnerError: errors.New("runner failed"),
	}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), svc, newRuntime(suite.T()))
	err := sr.Run()

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StatePreparing,
		events.StatePreparing,
	}, sr)
	suite.Assert().EqualError(err, "failed to create runner: runner failed")
}

func (suite *ServiceRunnerSuite) TestRunnerStageSkipped() {
	svc := &MockService{
		nilRunner: true,
	}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), svc, newRuntime(suite.T()))
	err := sr.Run()

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StatePreparing,
		events.StatePreparing,
	}, sr)
	suite.Assert().ErrorIs(err, system.ErrSkip)
}

func (suite *ServiceRunnerSuite) TestAbortOnCondition() {
	svc := &MockService{
		condition: conditions.WaitForFileToExist("/doesntexistever"),
	}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), svc, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		state := sr.AsProto().State
		if state != events.StateWaiting.String() {
			return retry.ExpectedErrorf("service should be waiting")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	sr.Shutdown()

	// a shutdown while waiting on the condition is a clean stop, not a failure
	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StateWaiting,
	}, sr)
}

func (suite *ServiceRunnerSuite) TestPostStateFail() {
	svc := &MockService{
		condition: conditions.None(),
		postError: errors.New("post failed"),
	}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), svc, newRuntime(suite.T()))

	errCh := make(chan error)
	runNotify := make(chan struct{})

	go func() {
		errCh <- sr.Run(runNotify)
	}()

	<-runNotify

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StateWaiting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
		events.StateFailed,
	}, sr)
}

func (suite *ServiceRunnerSuite) TestRunFail() {
	runner := &MockRunner{exitCh: make(chan error)}
	svc := &MockService{runner: runner}
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), svc, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	runner.exitCh <- errors.New("run failed")

	suite.Assert().EqualError(<-errCh, "failed running service: error running service: run failed")

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
	}, sr)
}

func (suite *ServiceRunnerSuite) TestFullFlowRestart() {
	sr := system.NewServiceRunner(system.Services(newRuntime(suite.T())), &MockService{
		condition: conditions.None(),
	}, newRuntime(suite.T()))

	errCh := make(chan error)

	go func() {
		errCh <- sr.Run()
	}()

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		state := sr.AsProto().State
		if state != events.StateRunning.String() {
			return retry.ExpectedErrorf("service should be running")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	notifyCh := make(chan struct{})

	go func() {
		errCh <- sr.Run(notifyCh)
	}()

	<-notifyCh

	suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(10*time.Millisecond)).Retry(func() error {
		state := sr.AsProto().State
		if state != events.StateRunning.String() {
			return retry.ExpectedErrorf("service should be running")
		}

		return nil
	}))

	select {
	case <-errCh:
		suite.Require().Fail("service running should be still running")
	default:
	}

	sr.Shutdown()

	suite.Assert().NoError(<-errCh)

	suite.assertStateSequence([]events.ServiceState{
		events.StateStarting,
		events.StateWaiting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
		events.StateStarting,
		events.StateWaiting,
		events.StatePreparing,
		events.StatePreparing,
		events.StateRunning,
	}, sr)
}

func TestServiceRunnerSuite(t *testing.T) {
	suite.Run(t, new(ServiceRunnerSuite))
}
