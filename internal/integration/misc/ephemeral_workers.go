// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api && integration_k8s

package misc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	corev1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// EphemeralWorkersSuite validates clusters where worker nodes (only) are fully
// ephemeral: STATE and EPHEMERAL on workers are tmpfs volumes, wiped on reboot,
// while control plane nodes keep disk-backed volumes.
//
// Run with a cluster created via `WITH_EPHEMERAL_WORKERS=true` and
// `INTEGRATION_TEST_RUN=misc.EphemeralWorkersSuite`.
type EphemeralWorkersSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements base.NamedSuite.
func (suite *EphemeralWorkersSuite) SuiteName() string {
	return "misc.EphemeralWorkersSuite"
}

// SetupTest sets up the test context.
func (suite *EphemeralWorkersSuite) SetupTest() {
	if !suite.EphemeralWorkers {
		suite.T().Skip("skipping: cluster does not have ephemeral workers (-talos.ephemeral-workers)")
	}

	if len(suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)) == 0 {
		suite.T().Skip("skipping: cluster has no worker nodes")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest cancels the test context.
func (suite *EphemeralWorkersSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestWorkerVolumesAreMemory asserts that worker volumes are memory-backed while
// control plane volumes are disk-backed.
func (suite *EphemeralWorkersSuite) TestWorkerVolumesAreMemory() {
	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker) {
		nodeCtx := client.WithNode(suite.ctx, node)

		for _, id := range []string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel} {
			vs, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, id)
			suite.Require().NoError(err, "worker %s volume %s", node, id)

			suite.Assert().Equal(block.VolumeTypeMemory.String(), vs.TypedSpec().Type.String(),
				"worker %s volume %s should be memory-backed", node, id)
			suite.Assert().Equal(block.VolumePhaseReady, vs.TypedSpec().Phase,
				"worker %s volume %s should be Ready", node, id)
		}
	}

	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		vs, err := safe.StateGetByID[*block.VolumeStatus](
			client.WithNode(suite.ctx, node), suite.Client.COSI, constants.EphemeralPartitionLabel)
		suite.Require().NoError(err, "controlplane %s volume %s", node, constants.EphemeralPartitionLabel)

		suite.Assert().NotEqual(block.VolumeTypeMemory.String(), vs.TypedSpec().Type.String(),
			"controlplane %s EPHEMERAL should be disk-backed", node)
	}
}

// TestWorkerDataWipedOnReboot writes a marker file under /var on a worker,
// reboots it and asserts the marker is gone while the node rejoins the cluster.
func (suite *EphemeralWorkersSuite) TestWorkerDataWipedOnReboot() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	const markerPath = "/var/ephemeral-marker-integration"

	workerIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	k8sNode := suite.getK8sNodeByIP(workerIP)

	suite.writeMarkerFile(workerIP, markerPath)

	suite.AssertRebooted(
		suite.ctx, workerIP, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)

	// the node must rejoin Kubernetes and become Ready again
	err := suite.WaitForK8sNodeReadinessStatus(suite.ctx, k8sNode.Name, func(status corev1.ConditionStatus) bool {
		return status == corev1.ConditionTrue
	})
	suite.Require().NoError(err, "worker %s (%s) did not become Ready after reboot", k8sNode.Name, workerIP)

	// ...and the marker file must be gone, proving /var was wiped
	suite.Require().False(suite.markerFileExists(workerIP, markerPath),
		"marker file %s survived reboot on ephemeral worker %s", markerPath, workerIP)

	// tmpfs volumes must be back in Ready phase after re-acquiring config
	for _, id := range []string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel} {
		vs, err := safe.StateGetByID[*block.VolumeStatus](
			client.WithNode(suite.ctx, workerIP), suite.Client.COSI, id)
		suite.Require().NoError(err, "worker %s volume %s", workerIP, id)

		suite.Assert().Equal(block.VolumePhaseReady, vs.TypedSpec().Phase,
			"worker %s volume %s should be Ready after reboot", workerIP, id)
	}
}

// getK8sNodeByIP maps a node internal IP to the Kubernetes Node object name.
func (suite *EphemeralWorkersSuite) getK8sNodeByIP(internalIP string) *corev1.Node {
	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, internalIP)
	suite.Require().NoError(err, "failed to find Kubernetes node for IP %s", internalIP)

	return k8sNode
}

// writeMarkerFile creates a marker file under /var of the given node using a privileged pod.
func (suite *EphemeralWorkersSuite) writeMarkerFile(nodeIP, path string) {
	podDef, err := suite.NewPrivilegedPod("ephemeral-marker-write")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(suite.getK8sNodeByIP(nodeIP).Name).
		WithHostVolumeMount("/var", "/host-var")

	suite.Require().NoError(podDef.Create(suite.ctx, 3*time.Minute))

	_, _, err = podDef.Exec(suite.ctx, fmt.Sprintf("echo %s > %s", nodeIP, path))
	suite.Require().NoError(err)

	suite.Require().NoError(podDef.Delete(suite.ctx))
}

// markerFileExists checks whether the given file exists on the node using a privileged pod.
func (suite *EphemeralWorkersSuite) markerFileExists(nodeIP, path string) bool {
	podDef, err := suite.NewPrivilegedPod("ephemeral-marker-check")
	suite.Require().NoError(err)

	podDef = podDef.WithNodeName(suite.getK8sNodeByIP(nodeIP).Name).
		WithHostVolumeMount("/var", "/host-var")

	suite.Require().NoError(podDef.Create(suite.ctx, 3*time.Minute))
	defer podDef.Delete(context.Background()) //nolint:errcheck

	stdout, _, err := podDef.Exec(suite.ctx, fmt.Sprintf("test -e %s && echo present || echo absent", path))
	suite.Require().NoError(err)

	return strings.TrimSpace(stdout) == "present"
}

func init() {
	allSuites = append(allSuites, new(EphemeralWorkersSuite))
}
