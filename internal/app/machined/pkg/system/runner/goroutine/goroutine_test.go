// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package goroutine_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	v1alpha1cfg "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
	log.Printf("state %s: %s", state, fmt.Sprintf(message, args...))
}

type GoroutineSuite struct {
	suite.Suite
	r runtime.Runtime

	tmpDir string

	loggingManager runtime.LoggingManager
}

func (suite *GoroutineSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.loggingManager = logging.NewFileLoggingManager(suite.tmpDir)

	s, err := v1alpha1.NewState()
	suite.Assert().NoError(err)

	cfg := &v1alpha1cfg.Config{}

	e := v1alpha1.NewEvents(100, 10)

	r := v1alpha1.NewRuntime(cfg, s, e, suite.loggingManager)

	suite.r = r
}

func (suite *GoroutineSuite) TearDownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *GoroutineSuite) TestRunSuccess() {
	r := goroutine.NewRunner(suite.r, "testsuccess",
		func(context.Context, runtime.Runtime, io.Writer) error {
			return nil
		}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *GoroutineSuite) TestRunFail() {
	r := goroutine.NewRunner(suite.r, "testfail",
		func(context.Context, runtime.Runtime, io.Writer) error {
			return errors.New("service failed")
		}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().EqualError(r.Run(MockEventSink), "service failed")
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *GoroutineSuite) TestRunPanic() {
	r := goroutine.NewRunner(suite.r, "testpanic",
		func(context.Context, runtime.Runtime, io.Writer) error {
			panic("service panic")
		}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	err := r.Run(MockEventSink)
	suite.Assert().Error(err)
	suite.Assert().Regexp("^panic in service: service panic.*", err.Error())
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *GoroutineSuite) TestStop() {
	r := goroutine.NewRunner(suite.r, "teststop",
		func(ctx context.Context, data runtime.Runtime, logger io.Writer) error {
			<-ctx.Done()

			return ctx.Err()
		}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	errCh := make(chan error)

	go func() {
		errCh <- r.Run(MockEventSink)
	}()

	time.Sleep(20 * time.Millisecond)

	select {
	case <-errCh:
		suite.Require().Fail("should not return yet")
	default:
	}

	suite.Assert().NoError(r.Stop())
	suite.Assert().NoError(<-errCh)
}

func (suite *GoroutineSuite) TestRunLogs() {
	r := goroutine.NewRunner(suite.r, "logtest",
		func(ctx context.Context, data runtime.Runtime, logger io.Writer) error {
			//nolint:errcheck
			_, _ = logger.Write([]byte("Test 1\nTest 2\n"))

			return nil
		}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))

	logFile, err := os.Open(filepath.Join(suite.tmpDir, "logtest.log"))
	suite.Assert().NoError(err)

	//nolint:errcheck
	defer logFile.Close()

	logContents, err := ioutil.ReadAll(logFile)
	suite.Assert().NoError(err)

	suite.Assert().Equal([]byte("Test 1\nTest 2\n"), logContents)
}

func TestGoroutineSuite(t *testing.T) {
	suite.Run(t, new(GoroutineSuite))
}
