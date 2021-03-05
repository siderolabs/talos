// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/process"
	ctrs "github.com/talos-systems/talos/internal/pkg/containers"
	"github.com/talos-systems/talos/internal/pkg/containers/cri"
	criclient "github.com/talos-systems/talos/internal/pkg/cri"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const (
	busyboxImage = "docker.io/library/busybox:1.30.1"
	// busyboxImageDigest = "sha256:64f5d945efcc0f39ab11b3cd4ba403cc9fefe1fa3613123ca016cf3708e8cafb".
	// pauseImage         = "k8s.gcr.io/pause:3.1".
	// pauseImageDigest   = "sha256:da86e6ba6ca197bf6bc5e9d900febd906b133eaa4750e6bed647b0fbe50ed43e".
)

func MockEventSink(state events.ServiceState, message string, args ...interface{}) {
}

type CRISuite struct {
	suite.Suite

	tmpDir string

	containerdRunner  runner.Runner
	containerdWg      sync.WaitGroup
	containerdAddress string

	client    *criclient.Client
	ctx       context.Context
	ctxCancel context.CancelFunc

	inspector ctrs.Inspector

	pods []string
}

func (suite *CRISuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

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
		runner.WithLoggingManager(logging.NewFileLoggingManager(suite.tmpDir)),
		runner.WithEnv([]string{"PATH=/bin:" + constants.PATH}),
	)
	suite.Require().NoError(suite.containerdRunner.Open(context.Background()))
	suite.containerdWg.Add(1)

	go func() {
		defer suite.containerdWg.Done()
		defer suite.containerdRunner.Close()      //nolint:errcheck
		suite.containerdRunner.Run(MockEventSink) //nolint:errcheck
	}()

	suite.client, err = criclient.NewClient("unix:"+suite.containerdAddress, 30*time.Second)
	suite.Require().NoError(err)

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)

	suite.inspector, err = cri.NewInspector(suite.ctx, cri.WithCRIEndpoint("unix:"+suite.containerdAddress))
	suite.Require().NoError(err)
}

func (suite *CRISuite) TearDownSuite() {
	suite.ctxCancel()
	suite.Require().NoError(suite.inspector.Close())

	suite.Require().NoError(suite.client.Close())

	suite.Require().NoError(suite.containerdRunner.Stop())
	suite.containerdWg.Wait()

	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *CRISuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)

	suite.pods = nil

	podSandboxConfig := &runtimeapi.PodSandboxConfig{
		Metadata: &runtimeapi.PodSandboxMetadata{
			Name:      "etcd-master-1",
			Uid:       "ed1a599a53090941c9b4025c7e3e883d",
			Namespace: "kube-system",
			Attempt:   0,
		},
		Labels: map[string]string{
			"io.kubernetes.pod.name":      "etcd-master-1",
			"io.kubernetes.pod.namespace": "kube-system",
		},
		LogDirectory: suite.tmpDir,
		Linux: &runtimeapi.LinuxPodSandboxConfig{
			SecurityContext: &runtimeapi.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtimeapi.NamespaceOption{
					Network: runtimeapi.NamespaceMode_NODE,
				},
			},
		},
	}

	podSandboxID, err := suite.client.RunPodSandbox(suite.ctx, podSandboxConfig, "")
	suite.Require().NoError(err)
	suite.pods = append(suite.pods, podSandboxID)
	suite.Require().Len(podSandboxID, 64)

	imageRef, err := suite.client.PullImage(suite.ctx, &runtimeapi.ImageSpec{
		Image: busyboxImage,
	}, podSandboxConfig)
	suite.Require().NoError(err)

	ctrID, err := suite.client.CreateContainer(suite.ctx, podSandboxID,
		&runtimeapi.ContainerConfig{
			Metadata: &runtimeapi.ContainerMetadata{
				Name: "etcd",
			},
			Labels: map[string]string{
				"io.kubernetes.container.name": "etcd",
				"io.kubernetes.pod.name":       "etcd-master-1",
				"io.kubernetes.pod.namespace":  "kube-system",
			},
			Annotations: map[string]string{
				"io.kubernetes.container.restartCount": "1",
			},
			Image: &runtimeapi.ImageSpec{
				Image: imageRef,
			},
			Command: []string{"/bin/sh", "-c", "sleep 3600"},
		}, podSandboxConfig)
	suite.Require().NoError(err)
	suite.Require().Len(ctrID, 64)

	err = suite.client.StartContainer(suite.ctx, ctrID)
	suite.Require().NoError(err)
}

func (suite *CRISuite) TearDownTest() {
	for _, pod := range suite.pods {
		suite.Require().NoError(suite.client.StopPodSandbox(suite.ctx, pod))
		suite.Require().NoError(suite.client.RemovePodSandbox(suite.ctx, pod))
	}

	suite.ctxCancel()
}

func (suite *CRISuite) TestPods() {
	pods, err := suite.inspector.Pods()
	suite.Require().NoError(err)

	suite.Require().Len(pods, 1)

	suite.Assert().Equal("kube-system/etcd-master-1", pods[0].Name)

	suite.Require().Len(pods[0].Containers, 2)

	suite.Assert().Equal(pods[0].Name, pods[0].Containers[0].Display)
	suite.Assert().Equal(pods[0].Name, pods[0].Containers[0].Name)
	suite.Assert().Equal("SANDBOX_READY", pods[0].Containers[0].Status)
	// suite.Assert().Equal(pauseImageDigest, pods[0].Containers[0].Digest)
	// suite.Assert().Equal(pauseImage, pods[0].Containers[0].Image)
	suite.Assert().True(pods[0].Containers[0].Pid > 0)

	suite.Assert().Equal("kube-system/etcd-master-1:etcd", pods[0].Containers[1].Display)
	suite.Assert().Equal("etcd", pods[0].Containers[1].Name)
	// suite.Assert().Equal(busyboxImage, pods[0].Containers[1].Image)
	// suite.Assert().Equal(busyboxImageDigest, pods[0].Containers[1].Digest)
	suite.Assert().Equal("CONTAINER_RUNNING", pods[0].Containers[1].Status)
	suite.Assert().Equal("1", pods[0].Containers[1].RestartCount)
	suite.Assert().True(pods[0].Containers[1].Pid > 0)
}

func (suite *CRISuite) TestContainer() {
	defer func() {
		r := recover()
		if r != nil {
			t := suite.T()
			t.Errorf("test panicked: %v %s", r, debug.Stack())
			t.FailNow()
		}
	}()

	container, err := suite.inspector.Container("kube-system/etcd-master-1")
	suite.Require().NoError(err)

	suite.Assert().Equal("kube-system/etcd-master-1", container.Display)
	suite.Assert().Equal(container.Display, container.Name)
	suite.Assert().Equal("SANDBOX_READY", container.Status)
	// suite.Assert().Equal(pauseImageDigest, container.Digest)
	suite.Assert().True(container.Pid > 0)

	container, err = suite.inspector.Container("kube-system/etcd-master-1:etcd")
	suite.Require().NoError(err)

	suite.Assert().Equal("kube-system/etcd-master-1:etcd", container.Display)
	suite.Assert().Equal("etcd", container.Name)
	suite.Assert().Equal("CONTAINER_RUNNING", container.Status)
	// suite.Assert().Equal(busyboxImageDigest, container.Image)
	// suite.Assert().Equal(busyboxImageDigest, container.Digest)
	suite.Assert().Equal("1", container.RestartCount)
	suite.Assert().True(container.Pid > 0)

	container, err = suite.inspector.Container("kube-system/etcd-master-1:etcd2")
	suite.Require().NoError(err)
	suite.Require().Nil(container)

	container, err = suite.inspector.Container("kube-system/etcd-master-2")
	suite.Require().NoError(err)
	suite.Require().Nil(container)

	container, err = suite.inspector.Container("talos")
	suite.Require().NoError(err)
	suite.Require().Nil(container)
}

func TestCRISuite(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("can't run the test as non-root")
	}

	_, err := os.Stat("/bin/containerd")
	if err != nil {
		t.Skip("containerd binary is not available, skipping the test")
	}

	suite.Run(t, new(CRISuite))
}
