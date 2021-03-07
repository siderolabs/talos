// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package process_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-cmd/pkg/cmd/proc/reaper"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
	log.Printf("state %s: %s", state, fmt.Sprintf(message, args...))
}

type ProcessSuite struct {
	suite.Suite

	tmpDir    string
	runReaper bool

	loggingManager runtime.LoggingManager
}

func (suite *ProcessSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.loggingManager = logging.NewFileLoggingManager(suite.tmpDir)

	if suite.runReaper {
		reaper.Run()
	}
}

func (suite *ProcessSuite) TearDownSuite() {
	if suite.runReaper {
		reaper.Shutdown()
	}

	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *ProcessSuite) TestRunSuccess() {
	r := process.NewRunner(false, &runner.Args{
		ID:          "test",
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *ProcessSuite) TestRunLogs() {
	r := process.NewRunner(false, &runner.Args{
		ID:          "logtest",
		ProcessArgs: []string{"/bin/sh", "-c", "echo -n \"Test 1\nTest 2\n\""},
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

func (suite *ProcessSuite) TestRunRestartFailed() {
	testFile := filepath.Join(suite.tmpDir, "talos-test")
	//nolint:errcheck
	_ = os.Remove(testFile)

	r := restart.New(process.NewRunner(false, &runner.Args{
		ID:          "restarter",
		ProcessArgs: []string{"/bin/sh", "-c", "echo \"ran\"; test -f " + testFile},
	}, runner.WithLoggingManager(suite.loggingManager)), restart.WithType(restart.UntilSuccess), restart.WithRestartInterval(time.Millisecond))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		suite.Assert().NoError(r.Run(MockEventSink))
	}()

	fetchLog := func() []byte {
		logFile, err := os.Open(filepath.Join(suite.tmpDir, "restarter.log"))
		suite.Assert().NoError(err)

		//nolint:errcheck
		defer logFile.Close()

		logContents, err := ioutil.ReadAll(logFile)
		suite.Assert().NoError(err)

		return logContents
	}

	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)

		if len(fetchLog()) > 20 {
			break
		}
	}

	f, err := os.Create(testFile)
	suite.Assert().NoError(err)
	suite.Assert().NoError(f.Close())

	wg.Wait()

	suite.Assert().True(len(fetchLog()) > 20)
}

func (suite *ProcessSuite) TestStopFailingAndRestarting() {
	testFile := filepath.Join(suite.tmpDir, "talos-test")
	//nolint:errcheck
	_ = os.Remove(testFile)

	r := restart.New(process.NewRunner(false, &runner.Args{
		ID:          "endless",
		ProcessArgs: []string{"/bin/sh", "-c", "test -f " + testFile},
	}, runner.WithLoggingManager(suite.loggingManager)), restart.WithType(restart.Forever), restart.WithRestartInterval(5*time.Millisecond))

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink)
	}()

	time.Sleep(40 * time.Millisecond)

	select {
	case <-done:
		suite.Assert().Fail("task should be running")

		return
	default:
	}

	f, err := os.Create(testFile)
	suite.Assert().NoError(err)
	suite.Assert().NoError(f.Close())

	time.Sleep(40 * time.Millisecond)

	select {
	case <-done:
		suite.Assert().Fail("task should be running")

		return
	default:
	}

	suite.Assert().NoError(r.Stop())
	<-done
}

func (suite *ProcessSuite) TestStopSigKill() {
	r := process.NewRunner(false, &runner.Args{
		ID:          "nokill",
		ProcessArgs: []string{"/bin/sh", "-c", "trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond),
	)

	suite.Assert().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink)
	}()

	time.Sleep(100 * time.Millisecond)

	suite.Assert().NoError(r.Stop())
	<-done
}

func TestProcessSuite(t *testing.T) {
	for _, runReaper := range []bool{true, false} {
		func(runReaper bool) {
			t.Run(fmt.Sprintf("runReaper=%v", runReaper), func(t *testing.T) { suite.Run(t, &ProcessSuite{runReaper: runReaper}) })
		}(runReaper)
	}
}
