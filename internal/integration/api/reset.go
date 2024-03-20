// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ResetSuite ...
type ResetSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ResetSuite) SuiteName() string {
	return "api.ResetSuite"
}

// SetupTest ...
func (suite *ResetSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	// make sure we abort at some point in time, but give enough room for Resets
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *ResetSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestResetNodeByNode Resets cluster node by node, waiting for health between Resets.
func (suite *ResetSuite) TestResetNodeByNode() {
	if suite.Capabilities().SecureBooted {
		// this is because in secure boot mode, the machine config is only applied and cannot be passed as kernel args
		suite.T().Skip("skipping as talos is explicitly trusted booted")
	}

	initNodeAddress := ""

	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == machine.TypeInit {
			initNodeAddress = node.IPs[0].String()

			break
		}
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	sort.Strings(nodes)

	for _, node := range nodes {
		if node == initNodeAddress {
			// due to the bug with etcd cluster build for the init node after Reset(), skip resetting first node
			// there's no problem if bootstrap API was used, so this check only protects legacy init nodes
			suite.T().Log("Skipping init node", node, "due to known issue with etcd")

			continue
		}

		suite.T().Log("Resetting node", node)

		preReset, err := suite.HashKubeletCert(suite.ctx, node)
		suite.Require().NoError(err)

		suite.AssertRebooted(
			suite.ctx, node, func(nodeCtx context.Context) error {
				// force reboot after reset, as this is the only mode we can test
				return base.IgnoreGRPCUnavailable(suite.Client.Reset(nodeCtx, true, true))
			}, 10*time.Minute,
		)

		suite.ClearConnectionRefused(suite.ctx, node)

		postReset, err := suite.HashKubeletCert(suite.ctx, node)
		suite.Require().NoError(err)

		suite.Assert().NotEqual(preReset, postReset, "reset should lead to new kubelet cert being generated")
	}
}

func (suite *ResetSuite) testResetNoGraceful(nodeType machine.Type) {
	if suite.Capabilities().SecureBooted {
		// this is because in secure boot mode, the machine config is only applied and cannot be passed as kernel args
		suite.T().Skip("skipping as talos is explicitly trusted booted")
	}

	node := suite.RandomDiscoveredNodeInternalIP(nodeType)

	suite.T().Logf("Resetting %s node !graceful %s", nodeType, node)

	preReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			// force reboot after reset, as this is the only mode we can test
			return base.IgnoreGRPCUnavailable(suite.Client.Reset(nodeCtx, false, true))
		}, 5*time.Minute,
	)

	suite.ClearConnectionRefused(suite.ctx, node)

	postReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Assert().NotEqual(preReset, postReset, "reset should lead to new kubelet cert being generated")
}

// TestResetNoGracefulWorker resets a worker in !graceful mode.
func (suite *ResetSuite) TestResetNoGracefulWorker() {
	suite.testResetNoGraceful(machine.TypeWorker)
}

// TestResetNoGracefulControlplane resets a control plane node in !graceful mode.
//
// As the node doesn't leave etcd, it relies on Talos to fix the problem on rejoin.
func (suite *ResetSuite) TestResetNoGracefulControlplane() {
	suite.testResetNoGraceful(machine.TypeControlPlane)
}

// TestResetWithSpecEphemeral resets only ephemeral partition on the node.
func (suite *ResetSuite) TestResetWithSpecEphemeral() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Log("Resetting node with spec=[EPHEMERAL]", node)

	preReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			// force reboot after reset, as this is the only mode we can test
			return base.IgnoreGRPCUnavailable(
				suite.Client.ResetGeneric(
					nodeCtx, &machineapi.ResetRequest{
						Reboot:   true,
						Graceful: true,
						SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
							{
								Label: constants.EphemeralPartitionLabel,
								Wipe:  true,
							},
						},
					},
				),
			)
		}, 5*time.Minute,
	)

	suite.ClearConnectionRefused(suite.ctx, node)

	postReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Assert().NotEqual(preReset, postReset, "reset should lead to new kubelet cert being generated")
}

// TestResetWithSpecState resets only state partition on the node.
//
// As ephemeral partition is not reset, so kubelet cert shouldn't change.
func (suite *ResetSuite) TestResetWithSpecState() {
	if suite.Capabilities().SecureBooted {
		// this is because in secure boot mode, the machine config is only applied and cannot be passed as kernel args
		suite.T().Skip("skipping as talos is explicitly trusted booted")
	}

	node := suite.RandomDiscoveredNodeInternalIP()

	suite.T().Log("Resetting node with spec=[STATE]", node)

	preReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	disks, err := suite.Client.Disks(client.WithNodes(suite.ctx, node))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(disks.Messages)

	userDisksToWipe := xslices.Map(
		xslices.Filter(disks.Messages[0].Disks, func(disk *storage.Disk) bool {
			return !disk.SystemDisk
		}),
		func(disk *storage.Disk) string {
			return disk.DeviceName
		},
	)

	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			// force reboot after reset, as this is the only mode we can test
			return base.IgnoreGRPCUnavailable(
				suite.Client.ResetGeneric(
					nodeCtx, &machineapi.ResetRequest{
						Reboot:   true,
						Graceful: true,
						SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
							{
								Label: constants.StatePartitionLabel,
								Wipe:  true,
							},
						},
						UserDisksToWipe: userDisksToWipe,
					},
				),
			)
		}, 5*time.Minute,
	)

	suite.ClearConnectionRefused(suite.ctx, node)

	postReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Assert().Equal(preReset, postReset, "ephemeral partition was not reset")
}

// TestResetDuringBoot resets the node multiple times, second reset is done
// before boot sequence is complete.
func (suite *ResetSuite) TestResetDuringBoot() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNodes(suite.ctx, node)

	suite.T().Log("Resetting node", node)

	for range 2 {
		bootID := suite.ReadBootIDWithRetry(nodeCtx, time.Minute*5)

		err := retry.Constant(5*time.Minute, retry.WithUnits(time.Millisecond*1000)).Retry(
			func() error {
				// force reboot after reset, as this is the only mode we can test
				return retry.ExpectedError(
					suite.Client.ResetGeneric(
						client.WithNodes(suite.ctx, node), &machineapi.ResetRequest{
							Reboot:   true,
							Graceful: true,
							SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
								{
									Label: constants.EphemeralPartitionLabel,
									Wipe:  true,
								},
							},
						},
					),
				)
			},
		)

		suite.Require().NoError(err)

		suite.AssertBootIDChanged(nodeCtx, bootID, node, time.Minute*5)
	}

	suite.WaitForBootDone(suite.ctx)
	suite.AssertClusterHealthy(suite.ctx)
}

func init() {
	allSuites = append(allSuites, new(ResetSuite))
}
