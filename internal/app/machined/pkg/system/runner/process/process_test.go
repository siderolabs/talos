// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package process_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
)

func MockEventSink(t *testing.T) func(state events.ServiceState, message string, args ...any) {
	return func(state events.ServiceState, message string, args ...any) {
		t.Logf("state %s: %s", state, fmt.Sprintf(message, args...))
	}
}

type ProcessSuite struct {
	suite.Suite

	tmpDir    string
	runReaper bool

	loggingManager runtime.LoggingManager
}

func (suite *ProcessSuite) SetupSuite() {
	suite.tmpDir = suite.T().TempDir()

	suite.loggingManager = logging.NewFileLoggingManager(suite.tmpDir)

	if suite.runReaper {
		reaper.Run()
	}
}

func (suite *ProcessSuite) TearDownSuite() {
	if suite.runReaper {
		reaper.Shutdown()
	}
}

func (suite *ProcessSuite) TestRunSuccess() {
	r := process.NewRunner(false, &runner.Args{
		ID:          "test",
		ProcessArgs: []string{"/bin/bash", "-c", "exit 0"},
	}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink(suite.T())))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *ProcessSuite) TestRunLogs() {
	r := process.NewRunner(false, &runner.Args{
		ID:          "logtest",
		ProcessArgs: []string{"/bin/bash", "-c", "echo -n \"Test 1\nTest 2\n\""},
	}, runner.WithLoggingManager(suite.loggingManager))

	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink(suite.T())))

	// the log file is written asynchronously, so we need to wait a bit
	suite.EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		logContents, err := os.ReadFile(filepath.Join(suite.tmpDir, "logtest.log"))
		asrt.NoError(err)

		asrt.Equal([]byte("Test 1\nTest 2\n"), logContents)
	}, time.Second, 10*time.Millisecond)
}

func (suite *ProcessSuite) TestRunRestartFailed() {
	testFile := filepath.Join(suite.tmpDir, "talos-test")
	//nolint:errcheck
	_ = os.Remove(testFile)

	r := restart.New(process.NewRunner(false, &runner.Args{
		ID:          "restarter",
		ProcessArgs: []string{"/bin/bash", "-c", "echo \"ran\"; test -f " + testFile},
	}, runner.WithLoggingManager(suite.loggingManager)), restart.WithType(restart.UntilSuccess), restart.WithRestartInterval(time.Millisecond))

	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()
		suite.Assert().NoError(r.Run(MockEventSink(suite.T())))
	}()

	fetchLog := func() []byte {
		logFile, err := os.Open(filepath.Join(suite.tmpDir, "restarter.log"))
		suite.Assert().NoError(err)

		//nolint:errcheck
		defer logFile.Close()

		logContents, err := io.ReadAll(logFile)
		suite.Assert().NoError(err)

		return logContents
	}

	for range 20 {
		time.Sleep(100 * time.Millisecond)

		if len(fetchLog()) > 20 {
			break
		}
	}

	f, err := os.Create(testFile)
	suite.Assert().NoError(err)
	suite.Assert().NoError(f.Close())

	wg.Wait()

	suite.Assert().GreaterOrEqual(len(fetchLog()), 20, fetchLog())
}

func (suite *ProcessSuite) TestStopFailingAndRestarting() {
	testFile := filepath.Join(suite.tmpDir, "talos-test")
	//nolint:errcheck
	_ = os.Remove(testFile)

	r := restart.New(process.NewRunner(false, &runner.Args{
		ID:          "endless",
		ProcessArgs: []string{"/bin/bash", "-c", "test -f " + testFile},
	}, runner.WithLoggingManager(suite.loggingManager)), restart.WithType(restart.Forever), restart.WithRestartInterval(5*time.Millisecond))

	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink(suite.T()))
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
		ProcessArgs: []string{"/bin/bash", "-c", "trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond),
	)

	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink(suite.T()))
	}()

	time.Sleep(100 * time.Millisecond)

	suite.Assert().NoError(r.Stop())
	<-done
}

func (suite *ProcessSuite) TestPriority() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test, need root privileges")
	}

	pidFile := filepath.Join(suite.tmpDir, "talos-test-pid-prio")
	//nolint:errcheck
	_ = os.Remove(pidFile)

	currentPriority, err := syscall.Getpriority(syscall.PRIO_PROCESS, os.Getpid())
	suite.Assert().NoError(err)

	if currentPriority <= 3 {
		suite.T().Skipf("skipping test, we already have low priority %d", currentPriority)
	}

	r := process.NewRunner(false, &runner.Args{
		ID:          "nokill",
		ProcessArgs: []string{"/bin/bash", "-c", "echo $BASHPID >> " + pidFile + "; trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond),
		runner.WithPriority(17),
	)
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink(suite.T()))
	}()

	time.Sleep(10 * time.Millisecond)

	pidString, err := os.ReadFile(pidFile)
	suite.Assert().NoError(err)

	pid, err := strconv.ParseUint(strings.Trim(string(pidString), "\r\n"), 10, 32)
	suite.Assert().NoError(err)

	currentPriority, err = syscall.Getpriority(syscall.PRIO_PROCESS, int(pid))
	suite.Assert().NoError(err)
	// 40..1 corresponds to -20..19 since system call interface must reserve -1 for error
	suite.Assert().Equalf(3, currentPriority, "process priority should be 3 (nice 17), got %d", currentPriority)

	time.Sleep(1000 * time.Millisecond)

	suite.Assert().NoError(r.Stop())
	<-done
}

func (suite *ProcessSuite) TestIOPriority() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test, need root privileges")
	}

	pidFile := filepath.Join(suite.tmpDir, "talos-test-pid-ionice")
	//nolint:errcheck
	_ = os.Remove(pidFile)

	//nolint:errcheck
	ioprio, _, _ := syscall.Syscall(syscall.SYS_IOPRIO_GET, uintptr(1), uintptr(os.Getpid()), 0)
	suite.Assert().NotEqual(-1, int(ioprio))

	if ioprio>>13 == runner.IoprioClassIdle {
		suite.T().Skipf("skipping test, we already have idle IO priority %d", ioprio)
	}

	r := process.NewRunner(false, &runner.Args{
		ID:          "nokill",
		ProcessArgs: []string{"/bin/bash", "-c", "echo $BASHPID >> " + pidFile + "; trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond),
		runner.WithIOPriority(runner.IoprioClassIdle, 6),
	)
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink(suite.T()))
	}()

	time.Sleep(10 * time.Millisecond)

	pidString, err := os.ReadFile(pidFile)
	suite.Assert().NoError(err)

	pid, err := strconv.ParseUint(strings.Trim(string(pidString), "\r\n"), 10, 32)
	suite.Assert().NoError(err)

	//nolint:errcheck
	ioprio, _, _ = syscall.Syscall(syscall.SYS_IOPRIO_GET, uintptr(1), uintptr(pid), 0)
	suite.Assert().NotEqual(-1, int(ioprio))
	suite.Assert().Equal(runner.IoprioClassIdle<<13+6, int(ioprio))

	time.Sleep(10 * time.Millisecond)

	suite.Assert().NoError(r.Stop())
	<-done
}

func (suite *ProcessSuite) TestSchedulingPolicy() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test, need root privileges")
	}

	pidFile := filepath.Join(suite.tmpDir, "talos-test-pid-sched")
	//nolint:errcheck
	_ = os.Remove(pidFile)

	pol, _, errno := syscall.Syscall(syscall.SYS_SCHED_GETSCHEDULER, uintptr(os.Getpid()), 0, 0)
	suite.Assert().Equal(0, int(errno))

	if pol == runner.SchedulingPolicyIdle {
		suite.T().Skipf("skipping test, we already have idle scheduling policy")
	}

	r := process.NewRunner(false, &runner.Args{
		ID:          "nokill",
		ProcessArgs: []string{"/bin/bash", "-c", "echo $BASHPID >> " + pidFile + "; trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond),
		runner.WithSchedulingPolicy(runner.SchedulingPolicyIdle),
	)
	suite.Assert().NoError(r.Open())

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink(suite.T()))
	}()

	time.Sleep(10 * time.Millisecond)

	pidString, err := os.ReadFile(pidFile)
	suite.Assert().NoError(err)

	pid, err := strconv.ParseUint(strings.Trim(string(pidString), "\r\n"), 10, 32)
	suite.Assert().NoError(err)

	pol, _, errno = syscall.Syscall(syscall.SYS_SCHED_GETSCHEDULER, uintptr(pid), 0, 0)
	suite.Assert().Equal(0, int(errno))
	suite.Assert().Equal(runner.SchedulingPolicyIdle, int(pol))

	time.Sleep(10 * time.Millisecond)

	suite.Assert().NoError(r.Stop())
	<-done
}

func TestProcessSuite(t *testing.T) {
	for _, runReaper := range []bool{true, false} {
		t.Run(fmt.Sprintf("runReaper=%v", runReaper),
			func(t *testing.T) {
				suite.Run(t, &ProcessSuite{runReaper: runReaper})
			},
		)
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
