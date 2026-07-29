// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/cel"
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

// TestVolumeWipeValidation verifies validation of both immediate and staged (on-reboot) wipe requests.
//
// An immediate wipe of a live system volume (e.g. EPHEMERAL) can't succeed while the node is
// running, as the volume is in use; that's exactly what --on-reboot is for. This test verifies
// both paths reject invalid requests synchronously, refuse to wipe an in-use volume immediately,
// and don't leave any partial effect behind when rejected.
func (suite *VolumeWipeSuite) TestVolumeWipeValidation() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing volume wipe request validation on %s", node)

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

	// a multi-ID request is validated as a whole: one unknown ID fails the entire call,
	// even though the other ID is valid
	err = suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{constants.StatePartitionLabel, "NOSUCHVOLUME"},
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.NotFound, client.StatusCode(err))

	// a staged (on-reboot) request for an unknown volume ID also fails synchronously, with no
	// partial effect on the staged wipe selectors persisted in META
	readStagedSelectors := func() (value string, exists bool) {
		metaKey, err := safe.StateGetByID[*runtimeres.MetaKey](nodeCtx, suite.Client.COSI, runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors))
		if err != nil {
			suite.Require().True(state.IsNotFoundError(err), "unexpected error reading staged wipe selectors: %s", err)

			return "", false
		}

		return metaKey.TypedSpec().Value, true
	}

	valueBefore, existedBefore := readStagedSelectors()

	err = suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{"NOSUCHVOLUME"},
		OnReboot:  true,
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.NotFound, client.StatusCode(err))

	valueAfter, existedAfter := readStagedSelectors()

	suite.Assert().Equal(existedBefore, existedAfter, "a failed staged wipe request must not create or remove the staged wipe selectors tag")
	suite.Assert().Equal(valueBefore, valueAfter, "a failed staged wipe request must not modify the staged wipe selectors tag")
}

// TestVolumeWipeStagedReboot verifies a staged (on-reboot) wipe of multiple volumes (EPHEMERAL, META) end-to-end.
//
// Staging writes the StagedWipeTargets META tag with a CEL selector per requested volume; on the next
// reboot the VolumeWipeController (running as part of the normal COSI controller runtime) consumes the
// tag, wipes each matching volume, and emits a VolumeWipeStatus resource. The volumes are then
// re-provisioned.
func (suite *VolumeWipeSuite) TestVolumeWipeStagedReboot() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	volumeIDs := []string{constants.EphemeralPartitionLabel, constants.MetaPartitionLabel}

	suite.T().Logf("staging wipe of %v on %s", volumeIDs, node)

	// Get the current partition UUIDs of the volumes to be wiped
	partitionUUIDs := make(map[string]string, len(volumeIDs))

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUIDs[vs.Metadata().ID()] = vs.TypedSpec().PartitionUUID
			asrt.NotEmpty(vs.TypedSpec().PartitionUUID)
		},
	)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: volumeIDs,
		OnReboot:  true,
	}))

	// the staged wipe tag should be written to META with a CEL selector embedding each partition UUID
	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
		func(metaKey *runtimeres.MetaKey, asrt *assert.Assertions) {
			for _, id := range volumeIDs {
				asrt.Contains(metaKey.TypedSpec().Value, partitionUUIDs[id])
			}
		},
	)

	suite.T().Logf("rebooting %s to apply the staged wipe", node)

	// reboot to apply the staged wipe; the controller wipes and reboots, which
	// AssertRebooted/WaitForBootDone tolerate (waits for the final MachineStageRunning).
	suite.AssertRebooted(
		suite.ctx, node,
		func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		},
		10*time.Minute,
		suite.CleanupFailedPods,
	)

	// the controller should have consumed (deleted) the staged wipe tag
	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
	)

	// all requested volumes should be re-provisioned and ready
	rtestutils.AssertResources(
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		volumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)
}

// TestVolumeWipeStagedSingleVolumeReboot verifies a staged (on-reboot) wipe of a single volume (STATE) end-to-end.
func (suite *VolumeWipeSuite) TestVolumeWipeStagedSingleVolumeReboot() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	volumeID := constants.StatePartitionLabel

	suite.T().Logf("staging wipe of %s on %s", volumeID, node)

	var partitionUUID string

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUID = vs.TypedSpec().PartitionUUID
			asrt.NotEmpty(partitionUUID)
		},
	)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{volumeID},
		OnReboot:  true,
	}))

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
		func(metaKey *runtimeres.MetaKey, asrt *assert.Assertions) {
			asrt.Contains(metaKey.TypedSpec().Value, partitionUUID)
		},
	)

	suite.T().Logf("rebooting %s to apply the staged wipe", node)

	suite.AssertRebooted(
		suite.ctx, node,
		func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		},
		10*time.Minute,
		suite.CleanupFailedPods,
	)

	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
	)

	rtestutils.AssertResource(
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)
}

// TestVolumeWipeStagedAccumulatesAcrossCalls verifies that staging a wipe for volume A, and later
// separately staging a wipe for volume B, results in both being wiped on the next reboot — the
// staged wipe selectors accumulate across calls rather than being overwritten by the latest call.
func (suite *VolumeWipeSuite) TestVolumeWipeStagedAccumulatesAcrossCalls() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	firstVolumeID, secondVolumeID := constants.EphemeralPartitionLabel, constants.MetaPartitionLabel

	partitionUUIDs := make(map[string]string, 2)

	rtestutils.AssertResources(
		nodeCtx, suite.T(), suite.Client.COSI,
		[]string{firstVolumeID, secondVolumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUIDs[vs.Metadata().ID()] = vs.TypedSpec().PartitionUUID
			asrt.NotEmpty(vs.TypedSpec().PartitionUUID)
		},
	)

	countStagedSelectors := func() int {
		metaKey, err := safe.StateGetByID[*runtimeres.MetaKey](nodeCtx, suite.Client.COSI, runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors))
		suite.Require().NoError(err)

		var selectors []cel.Expression

		suite.Require().NoError(json.Unmarshal([]byte(metaKey.TypedSpec().Value), &selectors))

		return len(selectors)
	}

	suite.T().Logf("staging wipe of %s on %s", firstVolumeID, node)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{firstVolumeID},
		OnReboot:  true,
	}))

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
		func(metaKey *runtimeres.MetaKey, asrt *assert.Assertions) {
			asrt.Contains(metaKey.TypedSpec().Value, partitionUUIDs[firstVolumeID])
		},
	)
	suite.Assert().Equal(1, countStagedSelectors(), "staging one volume should stage exactly one selector")

	suite.T().Logf("staging wipe of %s on %s in a separate call", secondVolumeID, node)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{secondVolumeID},
		OnReboot:  true,
	}))

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
		func(metaKey *runtimeres.MetaKey, asrt *assert.Assertions) {
			asrt.Contains(metaKey.TypedSpec().Value, partitionUUIDs[firstVolumeID])
			asrt.Contains(metaKey.TypedSpec().Value, partitionUUIDs[secondVolumeID])
		},
	)
	suite.Assert().Equal(2, countStagedSelectors(), "staging a second volume separately should accumulate, not overwrite, the staged selectors")

	suite.T().Logf("rebooting %s to apply both staged wipes", node)

	suite.AssertRebooted(
		suite.ctx, node,
		func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		},
		10*time.Minute,
		suite.CleanupFailedPods,
	)

	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
	)

	rtestutils.AssertResources(
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		[]string{firstVolumeID, secondVolumeID},
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)
}

// TestVolumeWipeImmediateSuccess verifies a real (non-rejected) immediate wipe of a single volume.
//
// META is the only built-in system volume that is never mounted while the node is running (it's
// accessed as a raw block device, not through the mount system), so it's the one safe target for a
// live, no-reboot wipe. Wipe() drops the partition entry itself, but VolumeStatus's state machine
// doesn't revisit an already-Ready volume to notice that, so the effect is verified via the
// DiscoveredVolume disappearing instead (the same pattern used for BlockDeviceWipe elsewhere).
//
// The dropped partition is only reprovisioned on the next boot, so this test finishes with a
// reboot: without it, META's VolumeStatus would stay stale (reporting the now-gone partition as
// Ready) for whatever other test happens to run next against the same node.
func (suite *VolumeWipeSuite) TestVolumeWipeImmediateSuccess() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	volumeID := constants.MetaPartitionLabel

	var location string

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			location = vs.TypedSpec().Location
			asrt.NotEmpty(location)
		},
	)

	suite.T().Logf("wiping %s on %s immediately", volumeID, node)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{volumeID},
		OnReboot:  false,
	}))

	rtestutils.AssertNoResource[*block.DiscoveredVolume](nodeCtx, suite.T(), suite.Client.COSI, filepath.Base(location))

	suite.T().Logf("rebooting %s to restore %s to a freshly reprovisioned state", node, volumeID)

	suite.AssertRebooted(
		suite.ctx, node,
		func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		},
		10*time.Minute,
		suite.CleanupFailedPods,
	)

	rtestutils.AssertResource(
		client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)
}

// TestVolumeWipeImmediateRejectsPartialBatch verifies that an immediate wipe request naming
// multiple volumes is rejected as a whole, and wipes none of them, if any one of them is mounted.
func (suite *VolumeWipeSuite) TestVolumeWipeImmediateRejectsPartialBatch() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	unmountedVolumeID := constants.MetaPartitionLabel
	mountedVolumeID := constants.EphemeralPartitionLabel

	var partitionUUIDBefore string

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		unmountedVolumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUIDBefore = vs.TypedSpec().PartitionUUID
			asrt.NotEmpty(partitionUUIDBefore)
		},
	)

	suite.T().Logf("wiping %s (mounted) and %s (unmounted) on %s immediately, expecting rejection", mountedVolumeID, unmountedVolumeID, node)

	err := suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{unmountedVolumeID, mountedVolumeID},
		OnReboot:  false,
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.FailedPrecondition, client.StatusCode(err))

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		unmountedVolumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(partitionUUIDBefore, vs.TypedSpec().PartitionUUID,
				"the valid, unmounted volume in the batch must not be touched when the whole request is rejected")
		},
	)
}

func init() {
	allSuites = append(allSuites, new(VolumeWipeSuite))
}
