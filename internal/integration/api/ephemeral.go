// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// EphemeralSuite validates ephemeral-mode behavior: STATE and EPHEMERAL are tmpfs
// volumes, and node state is wiped on reboot.
type EphemeralSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements base.NamedSuite.
func (suite *EphemeralSuite) SuiteName() string {
	return "api.EphemeralSuite"
}

// SetupTest sets up the test context.
func (suite *EphemeralSuite) SetupTest() {
	if !suite.EphemeralNode {
		suite.T().Skip("skipping: cluster is not running in ephemeral mode (-talos.ephemeral-node)")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping: provisioner is not qemu")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 20*time.Minute)
}

// TearDownTest cancels the test context.
func (suite *EphemeralSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestVolumesAreMemory asserts that both system volumes report VolumeTypeMemory.
func (suite *EphemeralSuite) TestVolumesAreMemory() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		for _, id := range []string{constants.StatePartitionLabel, constants.EphemeralPartitionLabel} {
			vs, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, id)
			suite.Require().NoError(err, "node %s volume %s", node, id)

			suite.Assert().Equal(
				block.VolumeTypeMemory.String(),
				vs.TypedSpec().Type.String(),
				"node %s volume %s should be memory-backed", node, id,
			)
			suite.Assert().Equal(
				block.VolumePhaseReady,
				vs.TypedSpec().Phase,
				"node %s volume %s should be Ready", node, id,
			)
		}
	}
}

// TestMountsAreTmpfs asserts /system/state and /var are tmpfs on every node.
func (suite *EphemeralSuite) TestMountsAreTmpfs() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		r, err := suite.Client.Read(nodeCtx, "/proc/self/mountinfo")
		suite.Require().NoError(err)

		mounts, err := parseMountInfo(r)
		suite.Require().NoError(r.Close())
		suite.Require().NoError(err)

		seen := map[string]string{}

		for _, m := range mounts {
			if m.mountPoint == constants.StateMountPoint || m.mountPoint == constants.EphemeralMountPoint {
				seen[m.mountPoint] = m.fsType
			}
		}

		suite.Assert().Equal("tmpfs", seen[constants.StateMountPoint], "node %s /system/state fstype", node)
		suite.Assert().Equal("tmpfs", seen[constants.EphemeralMountPoint], "node %s /var fstype", node)
	}
}

// TestRebootWipesState reboots each node and confirms tmpfs mounts are recreated.
func (suite *EphemeralSuite) TestRebootWipesState() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		suite.T().Log("rebooting ephemeral node", node)

		suite.AssertRebooted(
			suite.ctx, node, func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
			}, 10*time.Minute,
			suite.CleanupFailedPods,
		)
	}

	// Re-verify mounts after reboot; the cluster must re-acquire config and
	// bring up tmpfs volumes again.
	suite.TestMountsAreTmpfs()
	suite.TestVolumesAreMemory()
}

func init() {
	allSuites = append(allSuites, new(EphemeralSuite))
}
