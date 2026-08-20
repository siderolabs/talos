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

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
)

type RestartSuite struct {
	suite.Suite
}

type MockRunner struct {
	exitCh chan error
	times  int
}

func (m *MockRunner) Open() error {
	return nil
}

func (m *MockRunner) Close() error {
	close(m.exitCh)

	return nil
}

func (m *MockRunner) Run(ctx context.Context, eventSink events.Recorder, _ runner.OnStart) (runner.Status, error) {
	select {
	case err := <-m.exitCh:
		m.times++

		return runner.Status{Started: true}, err
	case <-ctx.Done():
		return runner.Status{Started: true}, nil
	}
}

func (m *MockRunner) String() string {
	return "MockRunner()"
}

func MockEventSink(state events.ServiceState, message string, args ...any) {
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
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	failed := errors.New("failed")

	go func() {
		mock.exitCh <- failed
	}()

	_, err := r.Run(context.Background(), MockEventSink, nil)
	suite.Assert().EqualError(err, failed.Error())
}

func (suite *RestartSuite) TestRunOnceStop() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.Once))
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)

	go func() {
		_, runErr := r.Run(ctx, MockEventSink, nil)

		errCh <- runErr
	}()

	cancel()
	suite.Assert().NoError(<-errCh)
}

func (suite *RestartSuite) TestRunUntilSuccess() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.UntilSuccess), restart.WithRestartInterval(time.Millisecond))
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	failed := errors.New("failed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)

	go func() {
		_, runErr := r.Run(ctx, MockEventSink, nil)

		errCh <- runErr
	}()

	mock.exitCh <- failed

	mock.exitCh <- failed

	mock.exitCh <- failed

	mock.exitCh <- nil

	suite.Assert().NoError(<-errCh)
	cancel()
	suite.Assert().Equal(4, mock.times)
}

func (suite *RestartSuite) TestRunForever() {
	mock := MockRunner{
		exitCh: make(chan error),
	}

	r := restart.New(&mock, restart.WithType(restart.Forever), restart.WithRestartInterval(time.Millisecond))
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	failed := errors.New("failed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error)

	go func() {
		_, runErr := r.Run(ctx, MockEventSink, nil)

		errCh <- runErr
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

	cancel()
	suite.Assert().NoError(<-errCh)
	suite.Assert().Equal(4, mock.times)
}

func TestRestartSuite(t *testing.T) {
	suite.Run(t, new(RestartSuite))
}
