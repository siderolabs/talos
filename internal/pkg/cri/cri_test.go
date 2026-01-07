// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup1"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/suite"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/pkg/cri"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	busyboxImage = "docker.io/library/busybox:1.30.1"
)

func MockEventSink(t *testing.T) func(state events.ServiceState, message string, args ...any) {
	return func(state events.ServiceState, message string, args ...any) {
		t.Logf(message, args...)
	}
}

type CRISuite struct {
	suite.Suite

	tmpDir string

	containerdRunner  runner.Runner
	containerdWg      sync.WaitGroup
	containerdAddress string

	client    *cri.Client
	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *CRISuite) SetupSuite() {
	if cgroups.Mode() == cgroups.Unified {
		suite.T().Skip("test doesn't pass under cgroupsv2")
	}

	var err error

	suite.tmpDir = suite.T().TempDir()

	stateDir, rootDir := filepath.Join(suite.tmpDir, "state"), filepath.Join(suite.tmpDir, "root")
	suite.Require().NoError(os.Mkdir(stateDir, 0o777))
	suite.Require().NoError(os.Mkdir(rootDir, 0o777))

	if cgroups.Mode() == cgroups.Unified {
		var (
			groupPath string
			manager   *cgroup2.Manager
		)

		groupPath, err = cgroup2.NestedGroupPath(suite.tmpDir)
		suite.Require().NoError(err)

		manager, err = cgroup2.NewManager(constants.CgroupMountPath, groupPath, &cgroup2.Resources{})
		suite.Require().NoError(err)

		defer manager.Delete() //nolint:errcheck
	} else {
		var manager cgroup1.Cgroup

		manager, err = cgroup1.New(cgroup1.NestedPath(suite.tmpDir), &specs.LinuxResources{})
		suite.Require().NoError(err)

		defer manager.Delete() //nolint:errcheck
	}

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
		runner.WithEnv([]string{constants.EnvPathWithBin}),
		runner.WithCgroupPath(suite.tmpDir),
	)
	suite.Require().NoError(suite.containerdRunner.Open())

	suite.containerdWg.Go(func() {
		defer suite.containerdRunner.Close() //nolint:errcheck

		suite.containerdRunner.Run(MockEventSink(suite.T())) //nolint:errcheck
	})

	suite.client, err = cri.NewClient("unix:"+suite.containerdAddress, 30*time.Second)
	suite.Require().NoError(err)
}

func (suite *CRISuite) TearDownSuite() {
	suite.ctxCancel()

	suite.Require().NoError(suite.client.Close())

	suite.Require().NoError(suite.containerdRunner.Stop())
	suite.containerdWg.Wait()
}

func (suite *CRISuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Second)
}

func (suite *CRISuite) TearDownTest() {
	suite.ctxCancel()
}

func (suite *CRISuite) TestRunSandboxContainer() {
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
	suite.Require().Len(podSandboxID, 64)

	imageRef, err := suite.client.PullImage(
		suite.ctx, &runtimeapi.ImageSpec{
			Image: busyboxImage,
		}, podSandboxConfig,
	)
	suite.Require().NoError(err)

	_, err = suite.client.ImageStatus(
		suite.ctx, &runtimeapi.ImageSpec{
			Image: imageRef,
		},
	)
	suite.Require().NoError(err)

	ctrID, err := suite.client.CreateContainer(
		suite.ctx, podSandboxID,
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
		}, podSandboxConfig,
	)
	suite.Require().NoError(err)
	suite.Require().Len(ctrID, 64)

	err = suite.client.StartContainer(suite.ctx, ctrID)
	suite.Require().NoError(err)

	_, err = suite.client.ContainerStats(suite.ctx, ctrID)
	suite.Require().NoError(err)

	_, _, err = suite.client.ContainerStatus(suite.ctx, ctrID, true)
	suite.Require().NoError(err)

	err = suite.client.StopContainer(suite.ctx, ctrID, 10)
	suite.Require().NoError(err)

	err = suite.client.RemoveContainer(suite.ctx, ctrID)
	suite.Require().NoError(err)

	err = suite.client.StopPodSandbox(suite.ctx, podSandboxID)
	suite.Require().NoError(err)

	err = suite.client.RemovePodSandbox(suite.ctx, podSandboxID)
	suite.Require().NoError(err)
}

func (suite *CRISuite) TestList() {
	pods, err := suite.client.ListPodSandbox(suite.ctx, &runtimeapi.PodSandboxFilter{})
	suite.Require().NoError(err)
	suite.Require().Len(pods, 0)

	containers, err := suite.client.ListContainers(suite.ctx, &runtimeapi.ContainerFilter{})
	suite.Require().NoError(err)
	suite.Require().Len(containers, 0)

	containerStats, err := suite.client.ListContainerStats(suite.ctx, &runtimeapi.ContainerStatsFilter{})
	suite.Require().NoError(err)
	suite.Require().Len(containerStats, 0)

	_, err = suite.client.ListImages(suite.ctx, &runtimeapi.ImageFilter{})
	suite.Require().NoError(err)
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
