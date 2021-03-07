// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package restart_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
)

type RestartSuite struct {
	suite.Suite
}

type MockRunner struct {
	exitCh  chan error
	times   int
	stop    chan struct{}
	stopped chan struct{}
}

func (m *MockRunner) Open(ctx context.Context) error {
	m.stop = make(chan struct{})
	m.stopped = make(chan struct{})

	return nil
}

func (m *MockRunner) Close() error {
	close(m.exitCh)

	return nil
}

func (m *MockRunner) Run(eventSink events.Recorder) error {
	defer close(m.stopped)

	select {
	case err := <-m.exitCh:
		m.times++

		return err
	case <-m.stop:
		return nil
	}
}

func (m *MockRunner) Stop() error {
	close(m.stop)

	<-m.stopped

	m.stop = make(chan struct{})
	m.stopped = make(chan struct{})

	return nil
}

func (m *MockRunner) String() string {
	return "MockRunner()"
}

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
	log.Printf("state %s: %s", state, fmt.Sprintf(message, args...))
}

func (suite *RestartSuite) TestString() {
	suite.Assert().Equal("Restart(UntilSuccess, MockRunner())", restart.New(&MockRunner{}, restart.WithType(restart.UntilSuccess)).String())
}

func (suite *RestartSuite) TestRunOnce() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.Once))
	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	failed := errors.New("failed")

	go func() {
		mock.exitCh <- failed
	}()

	suite.Assert().EqualError(r.Run(MockEventSink), failed.Error())
	suite.Assert().NoError(r.Stop())
}

func (suite *RestartSuite) TestRunOnceStop() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.Once))
	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	errCh := make(chan error)

	go func() {
		errCh <- r.Run(MockEventSink)
	}()

	suite.Assert().NoError(r.Stop())
	suite.Assert().NoError(<-errCh)
}

func (suite *RestartSuite) TestRunUntilSuccess() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.UntilSuccess), restart.WithRestartInterval(time.Millisecond))
	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	failed := errors.New("failed")
	errCh := make(chan error)

	go func() {
		errCh <- r.Run(MockEventSink)
	}()

	mock.exitCh <- failed
	mock.exitCh <- failed
	mock.exitCh <- failed
	mock.exitCh <- nil

	suite.Assert().NoError(<-errCh)
	suite.Assert().NoError(r.Stop())
	suite.Assert().Equal(4, mock.times)
}

func (suite *RestartSuite) TestRunForever() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.Forever), restart.WithRestartInterval(time.Millisecond))
	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	failed := errors.New("failed")
	errCh := make(chan error)

	go func() {
		errCh <- r.Run(MockEventSink)
	}()

	mock.exitCh <- failed
	mock.exitCh <- nil
	mock.exitCh <- failed
	mock.exitCh <- nil

	select {
	case <-errCh:
		suite.Assert().Fail("runner should be still running")
	default:
	}

	suite.Assert().NoError(r.Stop())
	suite.Assert().NoError(<-errCh)
	suite.Assert().Equal(4, mock.times)
}

func TestRestartSuite(t *testing.T) {
	suite.Run(t, new(RestartSuite))
}
