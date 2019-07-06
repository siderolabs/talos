/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containerd_test

import (
	"bytes"
	"context"
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
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	containerdrunner "github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	containerdNamespace = "talostest"
	busyboxImage        = "docker.io/library/busybox:latest"
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
	log.Printf("state %s: %s", state, fmt.Sprintf(message, args...))
}

type ContainerdSuite struct {
	suite.Suite

	tmpDir string

	containerdRunner runner.Runner
	containerdWg     sync.WaitGroup

	client *containerd.Client
	image  containerd.Image
}

// nolint: dupl
func (suite *ContainerdSuite) SetupSuite() {
	var err error

	args := &runner.Args{
		ID:          "containerd",
		ProcessArgs: []string{"/bin/containerd"},
	}

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.containerdRunner = process.NewRunner(
		&userdata.UserData{},
		args,
		runner.WithLogPath(suite.tmpDir),
		runner.WithEnv([]string{"PATH=/bin:" + constants.PATH}),
	)
	suite.Require().NoError(suite.containerdRunner.Open(context.Background()))
	suite.containerdWg.Add(1)
	go func() {
		defer suite.containerdWg.Done()
		defer func() { suite.Require().NoError(suite.containerdRunner.Close()) }()
		suite.Require().NoError(suite.containerdRunner.Run(MockEventSink))
	}()

	suite.client, err = containerd.New(constants.ContainerdAddress)
	suite.Require().NoError(err)

	ctx := namespaces.WithNamespace(context.Background(), containerdNamespace)

	suite.image, err = suite.client.Pull(ctx, busyboxImage, containerd.WithPullUnpack)
	suite.Require().NoError(err)
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

	// nolint: errcheck
	defer logFile.Close()

	logContents, err := ioutil.ReadAll(logFile)
	suite.Assert().NoError(err)

	return logContents
}

func (suite *ContainerdSuite) TestRunSuccess() {
	r := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "test",
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
	)

	suite.Require().NoError(r.Open(context.Background()))
	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r.Stop())
}

func (suite *ContainerdSuite) TestRunTwice() {
	r := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "runtwice",
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
	)

	suite.Require().NoError(r.Open(context.Background()))
	defer func() { suite.Assert().NoError(r.Close()) }()

	// running same container twice should be fine
	// (checks that containerd state is cleaned up properly)
	for i := 0; i < 2; i++ {
		suite.Assert().NoError(r.Run(MockEventSink))
		// calling stop when Run has finished is no-op
		suite.Assert().NoError(r.Stop())

		// TODO: workaround containerd (?) bug: https://github.com/docker/for-linux/issues/643
		time.Sleep(100 * time.Millisecond)
	}
}

func (suite *ContainerdSuite) TestContainerCleanup() {
	// create two runners with the same container ID
	//
	// open first runner, but don't close it; second runner should be
	// able to start the container by cleaning up container created by the first
	// runner
	r1 := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "cleanup1",
		ProcessArgs: []string{"/bin/sh", "-c", "exit 1"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
	)

	suite.Require().NoError(r1.Open(context.Background()))

	r2 := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "cleanup1",
		ProcessArgs: []string{"/bin/sh", "-c", "exit 0"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
	)
	suite.Require().NoError(r2.Open(context.Background()))
	defer func() { suite.Assert().NoError(r2.Close()) }()

	suite.Assert().NoError(r2.Run(MockEventSink))
	// calling stop when Run has finished is no-op
	suite.Assert().NoError(r2.Stop())
}

func (suite *ContainerdSuite) TestRunLogs() {
	r := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "logtest",
		ProcessArgs: []string{"/bin/sh", "-c", "echo -n \"Test 1\nTest 2\n\""},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
	)

	suite.Require().NoError(r.Open(context.Background()))
	defer func() { suite.Assert().NoError(r.Close()) }()

	suite.Assert().NoError(r.Run(MockEventSink))

	logFile, err := os.Open(filepath.Join(suite.tmpDir, "logtest.log"))
	suite.Assert().NoError(err)

	// nolint: errcheck
	defer logFile.Close()

	logContents, err := ioutil.ReadAll(logFile)
	suite.Assert().NoError(err)

	suite.Assert().Equal([]byte("Test 1\nTest 2\n"), logContents)
}

func (suite *ContainerdSuite) TestStopFailingAndRestarting() {
	testDir := filepath.Join(suite.tmpDir, "test")
	suite.Assert().NoError(os.Mkdir(testDir, 0770))

	testFile := filepath.Join(testDir, "talos-test")
	// nolint: errcheck
	_ = os.Remove(testFile)

	r := restart.New(containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "endless",
		ProcessArgs: []string{"/bin/sh", "-c", "test -f " + testFile + " && echo ok || (echo fail; false)"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithOCISpecOpts(
			oci.WithMounts([]specs.Mount{
				{Type: "bind", Destination: testDir, Source: testDir, Options: []string{"bind", "ro"}},
			}),
		),
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
		if bytes.Contains(suite.getLogContents("endless.log"), []byte("fail\n")) {
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
		if bytes.Contains(suite.getLogContents("endless.log"), []byte("ok\n")) {
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

	logContents := suite.getLogContents("endless.log")

	suite.Assert().Truef(bytes.Contains(logContents, []byte("ok\n")), "logContents doesn't contain success entry: %v", logContents)
	suite.Assert().Truef(bytes.Contains(logContents, []byte("fail\n")), "logContents doesn't contain fail entry: %v", logContents)
}

func (suite *ContainerdSuite) TestStopSigKill() {
	r := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "nokill",
		ProcessArgs: []string{"/bin/sh", "-c", "trap -- '' SIGTERM; while :; do :; done"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
		runner.WithGracefulShutdownTimeout(10*time.Millisecond))

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

func (suite *ContainerdSuite) TestImportSuccess() {
	reqs := []*containerdrunner.ImportRequest{
		{
			Path: "/usr/images/osd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("testtalos/osd"),
			},
		},
		{
			Path: "/usr/images/proxyd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("testtalos/proxyd"),
			},
		},
	}
	suite.Assert().NoError(containerdrunner.Import(containerdNamespace, reqs...))

	ctx := namespaces.WithNamespace(context.Background(), containerdNamespace)
	for _, imageName := range []string{"testtalos/osd", "testtalos/proxyd"} {
		image, err := suite.client.ImageService().Get(ctx, imageName)
		suite.Require().NoError(err)
		suite.Require().Equal(imageName, image.Name)
	}
}

func (suite *ContainerdSuite) TestImportFail() {
	reqs := []*containerdrunner.ImportRequest{
		{
			Path: "/usr/images/osd.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("testtalos/osd2"),
			},
		},
		{
			Path: "/usr/images/nothere.tar",
			Options: []containerd.ImportOpt{
				containerd.WithIndexName("testtalos/nothere"),
			},
		},
	}
	suite.Assert().Error(containerdrunner.Import(containerdNamespace, reqs...))
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
