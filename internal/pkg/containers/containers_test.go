/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containers_test

import (
	"context"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	containerdrunner "github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/process"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/containers"
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	containerdNamespace = "inspecttest"
	busyboxImage        = "docker.io/library/busybox:latest"
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
}

type ContainersSuite struct {
	suite.Suite

	tmpDir string

	containerdRunner runner.Runner
	containerdWg     sync.WaitGroup

	client *containerd.Client
	image  containerd.Image
}

// nolint: dupl
func (suite *ContainersSuite) SetupSuite() {
	var err error

	args := &runner.Args{
		ID:          "containerd",
		ProcessArgs: []string{"/rootfs/bin/containerd"},
	}

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.containerdRunner = process.NewRunner(
		&userdata.UserData{},
		args,
		runner.WithLogPath(suite.tmpDir),
		runner.WithEnv([]string{"PATH=/rootfs/bin:" + constants.PATH}),
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

func (suite *ContainersSuite) TearDownSuite() {
	suite.Require().NoError(suite.client.Close())

	suite.Require().NoError(suite.containerdRunner.Stop())
	suite.containerdWg.Wait()

	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *ContainersSuite) TestRunSuccess() {
	r := containerdrunner.NewRunner(&userdata.UserData{}, &runner.Args{
		ID:          "test",
		ProcessArgs: []string{"/bin/sh", "-c", "sleep 3600"},
	},
		runner.WithLogPath(suite.tmpDir),
		runner.WithNamespace(containerdNamespace),
		runner.WithContainerImage(busyboxImage),
	)

	suite.Require().NoError(r.Open(context.Background()))
	defer func() { suite.Assert().NoError(r.Close()) }()

	var wg sync.WaitGroup
	runningCh := make(chan struct{})

	wg.Add(1)
	go func() {
		runningSink := func(state events.ServiceState, message string, args ...interface{}) {
			if state == events.StateRunning {
				close(runningCh)
			}
		}

		defer wg.Done()
		suite.Assert().NoError(r.Run(runningSink))
	}()

	// wait for the container to be started actually
	<-runningCh

	i, err := containers.NewInspector(context.Background(), containerdNamespace)
	suite.Assert().NoError(err)

	pods, err := i.Pods()
	suite.Assert().NoError(err)
	suite.Assert().Len(pods, 1)
	suite.Assert().Equal("test", pods[0].Name)
	suite.Assert().Equal("", pods[0].Sandbox)
	suite.Assert().Len(pods[0].Containers, 1)
	suite.Assert().Equal("test", pods[0].Containers[0].Display)
	suite.Assert().Equal("test", pods[0].Containers[0].Name)
	suite.Assert().Equal("test", pods[0].Containers[0].ID)
	suite.Assert().Equal(busyboxImage, pods[0].Containers[0].Image)
	suite.Assert().Equal(containerd.Running, pods[0].Containers[0].Status.Status)
	suite.Assert().NotNil(pods[0].Containers[0].Metrics)

	suite.Assert().NoError(i.Close())

	suite.Assert().NoError(r.Stop())
	wg.Wait()
}

func TestContainersSuite(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("can't run the test as non-root")
	}
	_, err := os.Stat("/rootfs/bin/containerd")
	if err != nil {
		t.Skip("containerd binary is not available, skipping the test")
	}

	suite.Run(t, new(ContainersSuite))
}
