// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// K8sLessSuite provides basic verification of Talos with Kubernetes and etcd disabled.
type K8sLessSuite struct {
	BaseSuite

	track int
}

// SuiteName ...
func (suite *K8sLessSuite) SuiteName() string {
	return fmt.Sprintf("provision.K8sLessSuite-TR%d", suite.track)
}

// TestBasicFlows verifies cluster creation, basic health, operations, etc.
//
//nolint:gocyclo
func (suite *K8sLessSuite) TestBasicFlows() {
	options := clusterOptions{
		ClusterName: "k8s-less",

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       0,

		SourceKernelPath:    helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath: helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			DefaultSettings.CurrentVersion,
		),
		SourceVersion:    DefaultSettings.CurrentVersion,
		SourceK8sVersion: constants.DefaultKubernetesVersion,

		VersionContract: config.TalosVersionCurrent.DisableEtcd().DisableKubernetes(),
	}

	suite.setupCluster(options)

	ctx := suite.ctx
	c, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	suite.Run("machine status is ready", func() {
		for _, node := range suite.Cluster.Info().Nodes {
			nodeCtx := client.WithNode(ctx, node.IPs[0].String())

			rtestutils.AssertResource(nodeCtx, suite.T(), c.COSI, runtime.MachineStatusID, func(r *runtime.MachineStatus, asrt *assert.Assertions) {
				asrt.True(r.TypedSpec().Status.Ready)
				asrt.Empty(r.TypedSpec().Status.UnmetConditions)
				asrt.Equal(runtime.MachineStageRunning, r.TypedSpec().Stage)
			})
		}
	})

	suite.Run("no kubelet and etcd", func() {
		for _, node := range suite.Cluster.Info().Nodes {
			nodeCtx := client.WithNode(ctx, node.IPs[0].String())

			rtestutils.AssertNoResource[*v1alpha1.Service](nodeCtx, suite.T(), c.COSI, "etcd")
			rtestutils.AssertNoResource[*v1alpha1.Service](nodeCtx, suite.T(), c.COSI, "kubelet")
		}
	})

	suite.Run("cluster discovery is working", func() {
		for _, node := range suite.Cluster.Info().Nodes {
			nodeCtx := client.WithNode(ctx, node.IPs[0].String())

			rtestutils.AssertLength[*cluster.Member](nodeCtx, suite.T(), c.COSI, options.ControlplaneNodes)
		}
	})

	suite.Run("host DNS is enabled", func() {
		for _, node := range suite.Cluster.Info().Nodes {
			nodeCtx := client.WithNode(ctx, node.IPs[0].String())

			rtestutils.AssertResource(nodeCtx, suite.T(), c.COSI, network.HostDNSConfigID, func(r *network.HostDNSConfig, asrt *assert.Assertions) {
				asrt.True(r.TypedSpec().Enabled)
			})
		}
	})

	suite.Run("reboot", func() {
		nodeCtx := client.WithNode(ctx, suite.Cluster.Info().Nodes[0].IPs[0].String())

		oldBootID, err := safe.StateGetByID[*runtime.BootID](nodeCtx, c.COSI, runtime.BootIDID)
		suite.Require().NoError(err)

		suite.Require().NoError(c.Reboot(nodeCtx))

		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			newBootID, err := safe.StateGetByID[*runtime.BootID](nodeCtx, c.COSI, runtime.BootIDID)
			if !asrt.NoError(err) {
				return
			}

			asrt.NotEqual(oldBootID.TypedSpec().BootID, newBootID.TypedSpec().BootID)
		}, time.Minute, time.Second)

		suite.waitForClusterHealth()
	})

	suite.Run("reset", func() {
		nodeCtx := client.WithNode(ctx, suite.Cluster.Info().Nodes[1].IPs[0].String())

		oldBootID, err := safe.StateGetByID[*runtime.BootID](nodeCtx, c.COSI, runtime.BootIDID)
		suite.Require().NoError(err)

		oldIdentity, err := safe.StateGetByID[*cluster.Identity](nodeCtx, c.COSI, cluster.LocalIdentity)
		suite.Require().NoError(err)

		suite.Require().NoError(c.Reset(nodeCtx, true, true))

		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			newBootID, err := safe.StateGetByID[*runtime.BootID](nodeCtx, c.COSI, runtime.BootIDID)
			if !asrt.NoError(err) {
				return
			}

			asrt.NotEqual(oldBootID.TypedSpec().BootID, newBootID.TypedSpec().BootID)

			newIdentity, err := safe.StateGetByID[*cluster.Identity](nodeCtx, c.COSI, cluster.LocalIdentity)
			if !asrt.NoError(err) {
				return
			}

			asrt.NotEqual(oldIdentity.TypedSpec().NodeID, newIdentity.TypedSpec().NodeID)
		}, time.Minute, time.Second)

		suite.waitForClusterHealth()
	})

	suite.Run("upgrade", func() {
		nodeCtx := client.WithNode(ctx, suite.Cluster.Info().Nodes[2].IPs[0].String())

		ctrd := &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CRI,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		}

		pullClient, err := c.ImageClient.Pull(
			nodeCtx,
			&machine.ImageServicePullRequest{
				Containerd: ctrd,
				ImageRef:   options.SourceInstallerImage,
			},
		)
		suite.Require().NoError(err)

		var imgRef string

		for {
			msg, err := pullClient.Recv()
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)

			switch {
			case msg.GetName() != "":
				imgRef = msg.GetName()
			case msg.GetPullProgress() != nil:
				suite.T().Logf("pull progress: %s", msg.GetPullProgress().GetProgress().Fmt())
			}
		}

		suite.Require().NotEmpty(imgRef)

		upgradeClient, err := c.LifecycleClient.Upgrade(
			nodeCtx,
			&machine.LifecycleServiceUpgradeRequest{
				Containerd: ctrd,
				Source: &machine.InstallArtifactsSource{
					ImageName: imgRef,
				},
			},
		)
		suite.Require().NoError(err)

		for {
			msg, err := upgradeClient.Recv()
			if errors.Is(err, io.EOF) {
				break
			}

			suite.Require().NoError(err)

			if msg.GetProgress() != nil {
				suite.T().Logf("upgrade progress: %s", msg.GetProgress().Fmt())
			}
		}
	})
}

func init() {
	allSuites = append(
		allSuites,
		&K8sLessSuite{track: 3},
	)
}
