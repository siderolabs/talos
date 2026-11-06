// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api && integration_k8s

package misc

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// EphemeralSuite validates fully ephemeral single-node clusters: STATE and
// EPHEMERAL are tmpfs volumes, and node state is wiped on reboot.
//
// Run with a cluster created via `WITH_EPHEMERAL_NODE=true` and
// `INTEGRATION_TEST_RUN=misc.EphemeralSuite`.
type EphemeralSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName implements base.NamedSuite.
func (suite *EphemeralSuite) SuiteName() string {
	return "misc.EphemeralSuite"
}

// SetupTest sets up the test context.
func (suite *EphemeralSuite) SetupTest() {
	if !suite.EphemeralNode {
		suite.T().Skip("skipping: cluster is not running in ephemeral mode (-talos.ephemeral-node)")
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
			if m.mountPoint == constants.StateMountPoint ||
				m.mountPoint == constants.EphemeralMountPoint {
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

// mountInfoLine represents a single line from /proc/self/mountinfo.
//
// Format: (see https://www.kernel.org/doc/Documentation/filesystems/proc.txt)
// 36 35 98:0 /mnt1 /mnt2 rw,noatime master:1 - ext3 /dev/root rw,errors=continue
// (1) (2)(3)  (4)   (5)   (6)     (7)   (8) (9)    (10)         (11)
//
// We need fields (5) = mount point and (10) = filesystem type.
type mountInfoLine struct {
	mountPoint string
	fsType     string
}

func parseMountInfo(r io.Reader) ([]mountInfoLine, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read mountinfo: %w", err)
	}

	var result []mountInfoLine

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, " - ")
		if len(parts) != 2 {
			continue
		}

		before := strings.Fields(parts[0])
		after := strings.Fields(parts[1])

		if len(before) < 6 || len(after) < 2 {
			continue
		}

		result = append(result, mountInfoLine{
			mountPoint: before[4],
			fsType:     after[0],
		})
	}

	return result, nil
}

func init() {
	allSuites = append(allSuites, new(EphemeralSuite))
}
