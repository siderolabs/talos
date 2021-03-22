// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/google/uuid"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	containerdrunner "github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const (
	busyboxImage = "docker.io/library/busybox:latest"
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
	log.Printf("state %s: %s", state, fmt.Sprintf(message, args...))
}

type ContainerdSuite struct {
	suite.Suite

	tmpDir string

	loggingManager runtime.LoggingManager

	containerdNamespace string
	containerdRunner    runner.Runner
	containerdWg        sync.WaitGroup
	containerdAddress   string

	containerID string

	client *containerd.Client
	image  containerd.Image
}

func (suite *ContainerdSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.loggingManager = logging.NewFileLoggingManager(suite.tmpDir)

	stateDir, rootDir := filepath.Join(suite.tmpDir, "state"), filepath.Join(suite.tmpDir, "root")
	suite.Require().NoError(os.Mkdir(stateDir, 0o777))
	suite.Require().NoError(os.Mkdir(rootDir, 0o777))

	suite.containerdAddress = filepath.Join(suite.tmpDir, "run.sock")

	args := &runner.Args{
		ID: "containerd",
		ProcessArgs: []string{
			"/bin/containerd",
			"--address", suite.containerdAddress,
			"--state", stateDir,
			"--root", rootDir,
			"--config", constants.CRIContainerdConfig,
		},
	}

	suite.containerdRunner = process.NewRunner(
		false,
		args,
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithEnv([]string{"PATH=/bin:" + constants.PATH}),
	)
	suite.Require().NoError(suite.containerdRunner.Open(context.Background()))
	suite.containerdWg.Add(1)

	go func() {
		defer suite.containerdWg.Done()
		defer suite.containerdRunner.Close()      //nolint:errcheck
		suite.containerdRunner.Run(MockEventSink) //nolint:errcheck
	}()

	suite.client, err = containerd.New(suite.containerdAddress)
	suite.Require().NoError(err)

	namespace := ([16]byte)(uuid.New())
	suite.containerdNamespace = "talos" + hex.EncodeToString(namespace[:])

	ctx := namespaces.WithNamespace(context.Background(), suite.containerdNamespace)

	suite.image, err = suite.client.Pull(ctx, busyboxImage, containerd.WithPullUnpack)
	suite.Require().NoError(err)
}

func (suite *ContainerdSuite) SetupTest() {
	suite.containerID = uuid.New().String()
}

func (suite *ContainerdSuite) TearDownSuite() {
	suite.Require().NoError(suite.client.Close())

	suite.Require().NoError(suite.containerdRunner.Stop())
	suite.containerdWg.Wait()

	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *ContainerdSuite) getLogContents(filename string) []byte {
	logFile, err := os.Open(filepath.Join(suite.tmpDir, filename))
	suite.Assert().NoError(err)

	//nolint:errcheck
	defer logFile.Close()

	logContents, err := ioutil.ReadAll(logFile)
	suite.Assert().NoError(err)

	return logContents
}

func (suite *ContainerdSuite) TestRunSuccess() {
	r := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	)

	suite.Require().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *ContainerdSuite) TestRunTwice() {
	r := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	)

	suite.Require().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	// running same container twice should be fine
	// (checks that containerd state is cleaned up properly)
	for i := 0; i < 2; i++ {
		suite.Assert().NoError(r.Run(MockEventSink))
		// calling stop when Run has finished is no-op
		suite.Assert().NoError(r.Stop())

		if i == 0 {
			// wait a bit to let containerd clean up the state
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (suite *ContainerdSuite) TestContainerCleanup() {
	// create two runners with the same container ID
	//
	// open first runner, but don't close it; second runner should be
	// able to start the container by cleaning up container created by the first
	// runner
	r1 := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "exit 1"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	)

	suite.Require().NoError(r1.Open(context.Background()))

	r2 := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	)
	suite.Require().NoError(r2.Open(context.Background()))

	defer func() { suite.Assert().NoError(r2.Close()) }()

	suite.Assert().NoError(r2.Run(MockEventSink))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r2.Stop())
}

func (suite *ContainerdSuite) TestRunLogs() {
	r := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "echo -n \"Test 1\nTest 2\n\""},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	)

	suite.Require().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))

	logFile, err := os.Open(filepath.Join(suite.tmpDir, suite.containerID+".log"))
	suite.Assert().NoError(err)

	//nolint:errcheck
	defer logFile.Close()

	logContents, err := ioutil.ReadAll(logFile)
	suite.Assert().NoError(err)

	suite.Assert().Equal([]byte("Test 1\nTest 2\n"), logContents)
}

func (suite *ContainerdSuite) TestStopFailingAndRestarting() {
	testDir := filepath.Join(suite.tmpDir, "test")
	suite.Assert().NoError(os.Mkdir(testDir, 0o770))

	testFile := filepath.Join(testDir, "talos-test")
	//nolint:errcheck
	_ = os.Remove(testFile)

	r := restart.New(containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "test -f " + testFile + " && echo ok || (echo fail; false)"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithOCISpecOpts(
			oci.WithMounts([]specs.Mount{
				{Type: "bind", Destination: testDir, Source: testDir, Options: []string{"bind", "ro"}},
			}),
		),
		runner.WithContainerdAddress(suite.containerdAddress),
	),
		restart.WithType(restart.Forever),
		restart.WithRestartInterval(5*time.Millisecond),
	)

	suite.Require().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink)
	}()

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)

		if bytes.Contains(suite.getLogContents(suite.containerID+".log"), []byte("fail\n")) {
			break
		}
	}

	select {
	case err := <-done:
		suite.Assert().Failf("task should be running", "error: %s", err)

		return
	default:
	}

	f, err := os.Create(testFile)
	suite.Assert().NoError(err)
	suite.Assert().NoError(f.Close())

	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)

		if bytes.Contains(suite.getLogContents(suite.containerID+".log"), []byte("ok\n")) {
			break
		}
	}

	select {
	case err = <-done:
		suite.Assert().Failf("task should be running", "error: %s", err)

		return
	default:
	}

	suite.Assert().NoError(r.Stop())
	<-done

	logContents := suite.getLogContents(suite.containerID + ".log")

	suite.Assert().Truef(bytes.Contains(logContents, []byte("ok\n")), "logContents doesn't contain success entry: %v", logContents)
	suite.Assert().Truef(bytes.Contains(logContents, []byte("fail\n")), "logContents doesn't contain fail entry: %v", logContents)
}

func (suite *ContainerdSuite) TestStopSigKill() {
	r := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/sh", "-c", "trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond),
		runner.WithContainerdAddress(suite.containerdAddress),
	)

	suite.Require().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	done := make(chan error, 1)

	go func() {
		done <- r.Run(MockEventSink)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		suite.Assert().Fail("container should be still running")
	default:
	}

	time.Sleep(100 * time.Millisecond)

	suite.Assert().NoError(r.Stop())
	<-done
}

func (suite *ContainerdSuite) TestContainerStdin() {
	stdin := bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef}, 2000)

	r := containerdrunner.NewRunner(false, &runner.Args{
		ID:          suite.containerID,
		ProcessArgs: []string{"/bin/cat"},
	},
		runner.WithStdin(bytes.NewReader(stdin)),
		runner.WithLoggingManager(suite.loggingManager),
		runner.WithNamespace(suite.containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithContainerdAddress(suite.containerdAddress),
	)

	suite.Require().NoError(r.Open(context.Background()))

	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))

	logFile, err := os.Open(filepath.Join(suite.tmpDir, suite.containerID+".log"))
	suite.Assert().NoError(err)

	//nolint:errcheck
	defer logFile.Close()

	logContents, err := ioutil.ReadAll(logFile)
	suite.Assert().NoError(err)

	suite.Assert().Equal(stdin, logContents)
}

func TestContainerdSuite(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("can't run the test as non-root")
	}

	_, err := os.Stat("/bin/containerd")
	if err != nil {
		t.Skip("containerd binary is not available, skipping the test")
	}

	suite.Run(t, new(ContainerdSuite))
}
