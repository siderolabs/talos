// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
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

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	slices.Sort(nodes)

	for _, node := range nodes {
		suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
			Reboot:   true,
			Graceful: true,
		}, true)
	}
}

func (suite *ResetSuite) testResetNoGraceful(nodeType machine.Type) {
	if suite.Capabilities().SecureBooted {
		// this is because in secure boot mode, the machine config is only applied and cannot be passed as kernel args
		suite.T().Skip("skipping as talos is explicitly trusted booted")
	}

	node := suite.RandomDiscoveredNodeInternalIP(nodeType)

	suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
		Reboot:   true,
		Graceful: false,
	}, true)
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

	suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
		Reboot:   true,
		Graceful: true,
		SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
			{
				Label: constants.EphemeralPartitionLabel,
				Wipe:  true,
			},
		},
	}, true)
}

// TestResetWithSpecStateAndUserDisks resets state partition and user disks on the node.
//
// As ephemeral partition is not reset, so kubelet cert shouldn't change.
//
//nolint:gocyclo
func (suite *ResetSuite) TestResetWithSpecStateAndUserDisks() {
	if suite.Capabilities().SecureBooted {
		// this is because in secure boot mode, the machine config is only applied and cannot be passed as kernel args
		suite.T().Skip("skipping as talos is explicitly trusted booted")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("resetting STATE + user disk on node %s", node)

	config, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	// check if EPHEMERAL is encrypted locked to STATE
	var lockedToState bool

	if volume, ok := config.Volumes().ByName(constants.EphemeralPartitionLabel); ok && volume.Encryption() != nil {
		for _, key := range volume.Encryption().Keys() {
			if key.LockToSTATE() {
				lockedToState = true
			}
		}
	}

	disks, err := suite.Client.Disks(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(disks.Messages)

	userDisksToWipe := xslices.Map(
		xslices.Filter(disks.Messages[0].Disks, func(disk *storage.Disk) bool {
			switch {
			case disk.SystemDisk:
				return false
			case disk.Type == storage.Disk_UNKNOWN, disk.Type == storage.Disk_CD, disk.Type == storage.Disk_SD, disk.Type == storage.Disk_NVME:
				return false
			case disk.Readonly:
				return false
			case disk.BusPath == "/virtual":
				return false
			}

			return true
		}),
		func(disk *storage.Disk) string {
			return disk.DeviceName
		},
	)

	if !lockedToState {
		// if not locked to STATE, wipe will be successful
		suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
			Reboot:   true,
			Graceful: true,
			SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
				{
					Label: constants.StatePartitionLabel,
					Wipe:  true,
				},
			},
			UserDisksToWipe: userDisksToWipe,
		}, true)

		return
	}

	suite.T().Logf("verifying that EPHEMERAL partition would fail to unlock after reset, as it is locked to STATE")

	// if the EPHEMERAL partition is locked to STATE, it will fail to unlock after reset, so let's verify it
	suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
		Reboot:   true,
		Graceful: true,
		SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
			{
				Label: constants.StatePartitionLabel,
				Wipe:  true,
			},
		},
		UserDisksToWipe: userDisksToWipe,
	}, false)

	// wait for EPHEMERAL failure
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]string{constants.EphemeralPartitionLabel},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseFailed, vs.TypedSpec().Phase)
			asrt.Contains(vs.TypedSpec().ErrorMessage, "encryption key rejected")
		},
	)

	// now reset EPHEMERAL
	suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
		Reboot:   true,
		Graceful: false,
		SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
			{
				Label: constants.EphemeralPartitionLabel,
				Wipe:  true,
			},
		},
	}, true)
}

// TestResetDuringBoot resets the node while it is in boot sequence.
func (suite *ResetSuite) TestResetDuringBoot() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNodes(suite.ctx, node)

	suite.T().Log("rebooting node", node)

	bootIDBefore, err := suite.ReadBootID(nodeCtx)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Client.Reboot(nodeCtx))

	suite.AssertBootIDChanged(nodeCtx, bootIDBefore, node, 3*time.Minute)

	suite.ClearConnectionRefused(suite.ctx, node)

	// make sure EPHEMERAL is ready
	rtestutils.AssertResources(client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		[]string{constants.EphemeralPartitionLabel},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	suite.ResetNode(suite.ctx, node, &machineapi.ResetRequest{
		Reboot:   true,
		Graceful: true,
		SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
			{
				Label: constants.EphemeralPartitionLabel,
				Wipe:  true,
			},
		},
	}, true)
}

func init() {
	allSuites = append(allSuites, new(ResetSuite))
}
