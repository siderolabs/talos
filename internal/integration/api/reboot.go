// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"strings"
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
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
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

	// a previous test might have left a reboot in flight, so make sure the cluster is fully
	// booted before the test starts
	suite.WaitForBootDone(suite.ctx)
}

// TearDownTest ...
func (suite *RebootSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestRebootDefault reboots cluster node by node, waiting for health between reboots.
func (suite *RebootSuite) TestRebootDefault() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	// test with a single controlplane and a single worker node
	nodes := []string{
		suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane),
		suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker),
	}

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

// TestRebootKillsLeftoverWorkloads verifies that a workload process the graceful shutdown can't stop
// is killed before the volumes are torn down.
//
// The process escapes its container: it moves itself to a cgroup of its own under kubepods, so neither
// the kubelet nor the CRI know about it, ignores SIGTERM, and keeps the pod mount namespace (with the
// EPHEMERAL volume mounted) alive.
//
// With workload isolation, the container plane (and a host PID pod) runs in the sandbox PID namespace,
// so the process is killed along with the namespace when sandboxd is stopped, before killWorkloads runs.
// Without it (e.g. a cluster with Talos 1.13 config defaults), killWorkloads must kill the process.
func (suite *RebootSuite) TestRebootKillsLeftoverWorkloads() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	cfg, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	workloadIsolation := cfg.SecurityProfileConfig() != nil && cfg.SecurityProfileConfig().WorkloadIsolation()

	suite.T().Logf("workload isolation enabled on node %s: %v", node, workloadIsolation)

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	pod, err := suite.NewPrivilegedPod("straggler")
	suite.Require().NoError(err)

	pod = pod.WithNodeName(k8sNode.Name)

	suite.Require().NoError(pod.Create(suite.ctx, 5*time.Minute))

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.Assert().NoError(pod.Delete(cleanupCtx))
	})

	const stragglerCgroup = constants.CgroupKubepods + "/talos-straggler"

	// the host root is mounted at /host in the privileged pod
	_, _, err = pod.Exec(suite.ctx, fmt.Sprintf(
		`mkdir -p /host/sys/fs/cgroup/%[1]s && `+
			`nohup sh -c 'echo $$ > /host/sys/fs/cgroup/%[1]s/cgroup.procs && trap "" TERM && cd /host/var && exec sleep 2147483647' `+
			`</dev/null >/dev/null 2>&1 &`,
		stragglerCgroup,
	))
	suite.Require().NoError(err)

	// make sure the straggler is running before the reboot
	suite.Require().NoError(retry.Constant(30*time.Second, retry.WithUnits(time.Second)).Retry(func() error {
		stdout, _, err := pod.Exec(suite.ctx, fmt.Sprintf("cat /host/sys/fs/cgroup/%s/cgroup.procs", stragglerCgroup))
		if err != nil {
			return err
		}

		if strings.TrimSpace(stdout) == "" {
			return retry.ExpectedErrorf("straggler is not running yet")
		}

		return nil
	}))

	// watch the kernel log for the killWorkloads report and the sequence result while the node is shutting down
	dmesgCtx, dmesgCancel := context.WithCancel(nodeCtx)
	defer dmesgCancel()

	dmesgStream, err := suite.Client.Dmesg(dmesgCtx, true, false)
	suite.Require().NoError(err)

	logReader, err := client.ReadStream(dmesgStream)
	suite.Require().NoError(err)

	var sawKill, sawSequenceFailure atomic.Bool

	go func() {
		scanner := bufio.NewScanner(logReader)

		for scanner.Scan() {
			line := scanner.Text()

			switch {
			case strings.Contains(line, "killed leftover workload process") && strings.Contains(line, stragglerCgroup):
				suite.T().Log(line)

				sawKill.Store(true)
			case strings.Contains(line, "reboot sequence: failed"):
				suite.T().Log(line)

				sawSequenceFailure.Store(true)
			}
		}
	}()

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)

	dmesgCancel()

	// a failed sequence still reboots the node via the emergency path, so check it explicitly
	suite.Require().False(sawSequenceFailure.Load(), "the reboot sequence should not fail")

	if workloadIsolation {
		// the straggler is killed with the sandbox PID namespace, killWorkloads has nothing left to kill
		return
	}

	suite.Require().True(sawKill.Load(), "the straggler process should have been killed by killWorkloads")
}

// TestForcedReboot force-reboots cluster node by node,
// ensuring that the 'cleanup' phase/'stopAllPods' task doesn't run.
func (suite *RebootSuite) TestForcedReboot() { //nolint:gocyclo
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	// test with a single controlplane and a single worker node
	nodes := []string{
		suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane),
		suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker),
	}

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

// TestRebootMultipleTimes reboots a node, issues consequent reboots
// reboot should cancel boot sequence, and cancel another reboot.
func (suite *RebootSuite) TestRebootMultipleTimes() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Log("rebooting node", node)

	bootID := suite.ReadBootIDWithRetry(nodeCtx, time.Minute*5)

	suite.T().Logf("initial bootID for node %s: %s", node, bootID)

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

	suite.T().Logf("new bootID for node %s: %s", node, bootID)

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

	// the assertion above is satisfied by the first boot ID change, while another reboot might be
	// still in flight, so wait for the node to settle before asserting that it is fully booted
	bootID = suite.WaitForBootIDStable(nodeCtx, node, time.Minute*5)

	suite.T().Logf("after 2nd reboot bootID for node %s: %s", node, bootID)

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
				nodeCtx := client.WithNode(suite.ctx, node)

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

	respCh := multiplex.Unary(suite.ctx, nodes, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, base.IgnoreGRPCUnavailable(suite.Client.Reboot(ctx))
	})

	for resp := range respCh {
		suite.Require().NoError(resp.Err, "error sending reboot command to node %q", resp.Node)
	}

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				bootIDBeforeInterface, ok := initialBootID.Load(node)
				if !ok {
					return fmt.Errorf("bootID record not found for %q", node)
				}

				bootIDBefore := bootIDBeforeInterface.(string) //nolint:forcetypeassert

				nodeCtx := client.WithNode(suite.ctx, node)

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

	suite.ClearConnectionRefused(suite.ctx, nodes...)

	suite.WaitForBootDone(suite.ctx)

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
	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.State(), []string{userVolumeID},
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
