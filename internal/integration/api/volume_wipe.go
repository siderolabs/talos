// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VolumeWipeSuite verifies the VolumeWipe API (talosctl wipe volume), both
// immediate and staged (--on-reboot) modes.
type VolumeWipeSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *VolumeWipeSuite) SuiteName() string {
	return "api.VolumeWipeSuite"
}

// SetupTest ...
func (suite *VolumeWipeSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping volume wipe test for non-qemu provisioner")
	}

	// give enough room for the staged wipe + double reboot to complete
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *VolumeWipeSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestVolumeWipeImmediate verifies validation and the in-use rejection of an immediate wipe.
//
// An immediate wipe of a live system volume (e.g. EPHEMERAL) can't succeed while the node is
// running, as the volume is in use; that's exactly what --on-reboot is for. This test verifies
// the immediate path rejects invalid requests and refuses to wipe an in-use volume.
func (suite *VolumeWipeSuite) TestVolumeWipeImmediate() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing immediate volume wipe rejections on %s", node)

	// no volume IDs specified
	err := suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.InvalidArgument, client.StatusCode(err))

	// unknown volume ID
	err = suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{"NOSUCHVOLUME"},
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.NotFound, client.StatusCode(err))

	// immediate wipe of an in-use system volume is rejected (blocks on the parent-disk lock retry)
	err = suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{constants.EphemeralPartitionLabel},
		OnReboot:  false,
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.FailedPrecondition, client.StatusCode(err))
	suite.Assert().Contains(err.Error(), "retry with --on-reboot")
}

// TestVolumeWipeStagedReboot verifies a staged (on-reboot) wipe of EPHEMERAL end-to-end.
//
// Staging writes the StagedVolumesToWipe META tag; on the next reboot the WipeStagedVolumes boot
// task consumes the tag, wipes the volume, and reboots again. The volume is then re-provisioned.
func (suite *VolumeWipeSuite) TestVolumeWipeStagedReboot() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("staging EPHEMERAL wipe on %s", node)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{constants.EphemeralPartitionLabel},
		OnReboot:  true,
	}))

	// the staged wipe tag should be written to META
	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedVolumesToWipe),
		func(metaKey *runtimeres.MetaKey, asrt *assert.Assertions) {
			asrt.Contains(metaKey.TypedSpec().Value, constants.EphemeralPartitionLabel)
		},
	)

	suite.T().Logf("rebooting %s to apply the staged wipe", node)

	// reboot to apply the staged wipe; the boot task wipes and reboots a second time, which
	// AssertRebooted/WaitForBootDone tolerate (waits for the final MachineStageRunning).
	suite.AssertRebooted(
		suite.ctx, node,
		func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		},
		10*time.Minute,
		suite.CleanupFailedPods,
	)

	// the boot task should have consumed (deleted) the staged wipe tag
	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedVolumesToWipe),
	)

	// EPHEMERAL should be re-provisioned and ready
	rtestutils.AssertResources(
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		[]string{constants.EphemeralPartitionLabel},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)
}

func init() {
	allSuites = append(allSuites, new(VolumeWipeSuite))
}
