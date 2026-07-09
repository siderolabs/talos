// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// RebootSuite ...
type RebootSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *RebootSuite) SuiteName() string {
	return "api.RebootSuite"
}

// SetupTest ...
func (suite *RebootSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	// make sure we abort at some point in time, but give enough room for reboots
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *RebootSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestRebootNodeByNode reboots cluster node by node, waiting for health between reboots.
func (suite *RebootSuite) TestRebootNodeByNode() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		suite.T().Log("rebooting node", node)

		suite.AssertRebooted(
			suite.ctx, node, func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
			}, 10*time.Minute,
			suite.CleanupFailedPods,
		)
	}
}

// TestForcedReboot force-reboots cluster node by node,
// ensuring that the 'cleanup' phase/'stopAllPods' task doesn't run.
func (suite *RebootSuite) TestForcedReboot() { //nolint:gocyclo
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		suite.T().Log("force rebooting node", node)

		nodeCtx := client.WithNode(suite.ctx, node)

		var (
			sawStopAllPods  atomic.Bool
			sawCleanupPhase atomic.Bool
		)

		// watch events so we can verify graceful teardown did not happen
		watchCtx, watchCancel := context.WithCancel(nodeCtx)
		eventsCh := make(chan client.EventResult)
		suite.Require().NoError(suite.Client.EventsWatchV2(watchCtx, eventsCh))

		go func() {
			for {
				select {
				case <-watchCtx.Done():
					return
				case ev := <-eventsCh:
					if ev.Error != nil {
						continue
					}

					switch msg := ev.Event.Payload.(type) {
					case *machineapi.TaskEvent:
						if msg.GetTask() == "stopAllPods" {
							sawStopAllPods.Store(true)
						}
					case *machineapi.PhaseEvent:
						if msg.GetPhase() == "cleanup" {
							sawCleanupPhase.Store(true)
						}
					}
				}
			}
		}()

		suite.AssertRebooted(
			suite.ctx, node, func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx, client.WithForce))
			}, 10*time.Minute,
			suite.CleanupFailedPods,
		)

		watchCancel()

		suite.Require().Falsef(sawCleanupPhase.Load(), "cleanup phase must not run during forced reboot")
		suite.Require().Falsef(sawStopAllPods.Load(), "stopAllPods task must not run during forced reboot")
	}

	suite.WaitForBootDone(suite.ctx)
}

// TestRebootMultiple reboots a node, issues consequent reboots
// reboot should cancel boot sequence, and cancel another reboot.
func (suite *RebootSuite) TestRebootMultiple() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNodes(suite.ctx, node)

	bootID := suite.ReadBootIDWithRetry(nodeCtx, time.Minute*5)

	// Issue reboot.
	suite.Require().NoError(base.IgnoreGRPCUnavailable(
		suite.Client.Reboot(nodeCtx),
	))

	// Issue reboot once again and wait for node to get a new boot id.
	suite.Require().NoError(base.IgnoreGRPCUnavailable(
		suite.Client.Reboot(nodeCtx),
	))

	suite.AssertBootIDChanged(nodeCtx, bootID, node, time.Minute*7)

	bootID = suite.ReadBootIDWithRetry(nodeCtx, time.Minute*5)

	suite.Require().NoError(retry.Constant(time.Second * 5).Retry(func() error {
		// Issue reboot while the node is still booting.
		err := suite.Client.Reboot(nodeCtx)
		if err != nil {
			return retry.ExpectedError(err)
		}

		// Reboot again and wait for cluster to become healthy.
		suite.Require().NoError(base.IgnoreGRPCUnavailable(
			suite.Client.Reboot(nodeCtx),
		))

		return nil
	}))

	suite.AssertBootIDChanged(nodeCtx, bootID, node, time.Minute*7)
	suite.WaitForBootDone(suite.ctx)
}

// TestRebootAllNodes reboots all cluster nodes at the same time.
//
//nolint:gocyclo
func (suite *RebootSuite) TestRebootAllNodes() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	errCh := make(chan error, len(nodes))

	var initialBootID sync.Map

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				nodeCtx := client.WithNodes(suite.ctx, node)

				// read boot_id before reboot
				bootIDBefore, err := suite.ReadBootID(nodeCtx)
				if err != nil {
					return fmt.Errorf("error reading initial bootID (node %q): %w", node, err)
				}

				initialBootID.Store(node, bootIDBefore)

				return nil
			}()
		}(node)
	}

	for range nodes {
		suite.Require().NoError(<-errCh)
	}

	allNodesCtx := client.WithNodes(suite.ctx, nodes...)

	err := base.IgnoreGRPCUnavailable(suite.Client.Reboot(allNodesCtx))

	suite.Require().NoError(err)

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				bootIDBeforeInterface, ok := initialBootID.Load(node)
				if !ok {
					return fmt.Errorf("bootID record not found for %q", node)
				}

				bootIDBefore := bootIDBeforeInterface.(string) //nolint:forcetypeassert

				nodeCtx := client.WithNodes(suite.ctx, node)

				return retry.Constant(10 * time.Minute).Retry(
					func() error {
						requestCtx, requestCtxCancel := context.WithTimeout(nodeCtx, 5*time.Second)
						defer requestCtxCancel()

						bootIDAfter, err := suite.ReadBootID(requestCtx)
						if err != nil {
							// API might be unresponsive during reboot
							return retry.ExpectedErrorf("error reading bootID for node %q: %w", node, err)
						}

						if bootIDAfter == bootIDBefore {
							// bootID should be different after reboot
							return retry.ExpectedErrorf(
								"bootID didn't change for node %q: before %s, after %s",
								node,
								bootIDBefore,
								bootIDAfter,
							)
						}

						return nil
					},
				)
			}()
		}(node)
	}

	for range nodes {
		suite.Assert().NoError(<-errCh)
	}

	if suite.Cluster != nil {
		// without cluster state we can't do deep checks, but basic reboot test still works
		// NB: using `ctx` here to have client talking to init node by default
		suite.AssertClusterHealthy(suite.ctx)
	}
}

// TestRebootWithFailingUserVolume verifies that a user volume whose allocation fails
// (it references a disk that does not exist) does not block the reboot sequence.
//
// The reboot sequence tears down the volume lifecycle; a failing user volume that is
// never provisioned must not hold that teardown up, otherwise the node fails to reboot.
func (suite *RebootSuite) TestRebootWithFailingUserVolume() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	volumeID := fmt.Sprintf("badvol%04x", rand.Int31())
	userVolumeID := constants.UserVolumePrefix + volumeID

	suite.T().Logf("creating a failing user volume %q on node %s", volumeID, node)

	doc := blockcfg.NewUserVolumeConfigV1Alpha1()
	doc.MetaName = volumeID
	// selector references a disk that does not exist, so the volume can never be allocated
	doc.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(`disk.dev_path == "/dev/does-not-exist"`, celenv.DiskLocator()),
	)
	doc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
	doc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("1GiB")

	suite.PatchMachineConfig(nodeCtx, doc)
	defer suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, volumeID)

	// the user volume should fail to allocate (no disk matched the selector)
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{userVolumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseFailed, vs.TypedSpec().Phase)
		},
	)

	// reboot must complete despite the failing user volume
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)

	suite.WaitForBootDone(suite.ctx)
}

func init() {
	allSuites = append(allSuites, new(RebootSuite))
}
