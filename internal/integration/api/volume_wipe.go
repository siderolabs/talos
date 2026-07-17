// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
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
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
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

	// META is never a legal wipe target, and it poisons the whole batch (see TestVolumeWipeMetaRejected)
	err = suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{constants.EphemeralPartitionLabel, constants.MetaPartitionLabel},
		OnReboot:  true,
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.InvalidArgument, client.StatusCode(err))

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

// TestVolumeWipeStagedReboot verifies a staged (on-reboot) wipe of multiple volumes (EPHEMERAL, STATE) end-to-end.
//
// Staging writes the StagedWipeTargets META tag with a CEL selector per requested volume; on the next
// reboot the VolumeWipeController (running as part of the normal COSI controller runtime) consumes the
// tag, wipes each matching volume, and emits a VolumeWipeStatus resource. The volumes are then
// re-provisioned.
//
// Wiping STATE discards the machine config stored there; the QEMU provisioner passes `talos.config=`
// on the kernel cmdline, so the node re-acquires it from the platform source on the next boot. Any
// config patch applied to this worker by an earlier test is lost in the process.
func (suite *VolumeWipeSuite) TestVolumeWipeStagedReboot() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	volumeIDs := []string{constants.EphemeralPartitionLabel, constants.StatePartitionLabel}

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

	firstVolumeID, secondVolumeID := constants.EphemeralPartitionLabel, constants.StatePartitionLabel

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

// TestVolumeWipeMetaRejected verifies that META is refused as a wipe target in both modes.
//
// META can't be wiped: it carries the staged wipe instructions themselves, and it is the only
// volume provisioned before the wipe runs (VolumeConfigController exempts it from the
// VolumeWipeStatus gate, since the selectors have to be readable first). Wiping it drops a
// partition which is already provisioned and in use, and VolumeStatus's state machine never
// revisits an already-Ready volume to notice — so META would stay stale-Ready, reporting a
// device path that no longer exists, and every later META write would fail with ENOENT for the
// rest of that boot.
func (suite *VolumeWipeSuite) TestVolumeWipeMetaRejected() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	volumeID := constants.MetaPartitionLabel

	var partitionUUIDBefore string

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUIDBefore = vs.TypedSpec().PartitionUUID
			asrt.NotEmpty(partitionUUIDBefore)
		},
	)

	for _, onReboot := range []bool{false, true} {
		suite.T().Logf("wiping %s on %s (on_reboot=%v), expecting rejection", volumeID, node, onReboot)

		err := suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
			VolumeIds: []string{volumeID},
			OnReboot:  onReboot,
		})
		suite.Require().Error(err)
		suite.Assert().Equal(codes.InvalidArgument, client.StatusCode(err))
		suite.Assert().Contains(err.Error(), "can't be wiped")
	}

	// META must be untouched, and no wipe may have been staged for the next boot
	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(partitionUUIDBefore, vs.TypedSpec().PartitionUUID, "META must not be wiped")
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
	)
}

// TestVolumeWipeImmediateRejectsPartialBatch verifies that an immediate wipe request naming
// multiple volumes is rejected as a whole, and wipes none of them, if any one of them is mounted.
func (suite *VolumeWipeSuite) TestVolumeWipeImmediateRejectsPartialBatch() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	otherVolumeID := constants.StatePartitionLabel
	mountedVolumeID := constants.EphemeralPartitionLabel

	var partitionUUIDBefore string

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		otherVolumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUIDBefore = vs.TypedSpec().PartitionUUID
			asrt.NotEmpty(partitionUUIDBefore)
		},
	)

	suite.T().Logf("wiping %s (mounted) and %s on %s immediately, expecting rejection", mountedVolumeID, otherVolumeID, node)

	err := suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{otherVolumeID, mountedVolumeID},
		OnReboot:  false,
	})
	suite.Require().Error(err)
	suite.Assert().Equal(codes.FailedPrecondition, client.StatusCode(err))

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		otherVolumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(partitionUUIDBefore, vs.TypedSpec().PartitionUUID,
				"the other volume in the batch must not be touched when the whole request is rejected")
		},
	)
}

// TestVolumeWipeUserVolumeRejected verifies that a user volume is not a wipe target in either mode.
//
// Wiping is for system volumes: the volume manager re-provisions those from the machine config, while
// a user volume's contents are the user's own and are dropped by removing its config document instead.
// Covered here as well as in the unit tests because only a real node produces a user volume with the
// prefixed ID the API is actually asked about (`u-<name>`).
func (suite *VolumeWipeSuite) TestVolumeWipeUserVolumeRejected() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	const volumeName = "wipe-reject"

	volumeID := constants.UserVolumePrefix + volumeName

	suite.T().Logf("declaring user volume %q on %s", volumeName, node)

	// Directory-backed: it needs no spare disk, so this runs on any cluster.
	volumeDoc := blockcfg.NewUserVolumeConfigV1Alpha1()
	volumeDoc.MetaName = volumeName
	volumeDoc.VolumeType = new(block.VolumeTypeDirectory)

	suite.T().Cleanup(func() {
		cleanupCtx := client.WithNode(context.Background(), node)

		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, blockcfg.UserVolumeConfigKind, volumeName)
	})

	suite.PatchMachineConfig(nodeCtx, volumeDoc)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	for _, onReboot := range []bool{false, true} {
		suite.T().Logf("wiping %s on %s (on_reboot=%v), expecting rejection", volumeID, node, onReboot)

		err := suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
			VolumeIds: []string{volumeID},
			OnReboot:  onReboot,
		})
		suite.Require().Error(err)
		suite.Assert().Equal(codes.InvalidArgument, client.StatusCode(err))
		suite.Assert().Contains(err.Error(), "not a system volume")
	}

	// The volume is untouched, and nothing was staged for the next boot.
	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		volumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
		},
	)

	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		nodeCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
	)
}

// TestVolumeWipeStagedReshapeEphemeral covers the reason a staged wipe exists: reshaping system
// volumes across a reboot.
func (suite *VolumeWipeSuite) TestVolumeWipeStagedReshapeEphemeral() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	const (
		userVolumeName = "reshaped"
		gib            = 1024 * 1024 * 1024
		// What EPHEMERAL gives up, and what the user volume is then carved from. One GiB of slack
		// between the two absorbs partition alignment either side.
		shrinkBy          = 2 * gib
		userVolumeMinSize = 512 * 1024 * 1024
		userVolumeMaxSize = 1 * gib
		// Below this there is nothing to give up: the ephemeral-min-max test patch already asks for a
		// 4GB minimum, so shrinking past that would be reshaping into an unusable volume.
		minSizeToShrink = 4*gib + shrinkBy
	)

	userVolumeID := constants.UserVolumePrefix + userVolumeName

	var (
		partitionUUIDBefore string
		sizeBefore          uint64
	)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		constants.EphemeralPartitionLabel,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			partitionUUIDBefore = vs.TypedSpec().PartitionUUID
			sizeBefore = vs.TypedSpec().Size

			asrt.NotEmpty(partitionUUIDBefore)
			asrt.NotZero(sizeBefore)
		},
	)

	if sizeBefore < minSizeToShrink {
		suite.T().Skipf("EPHEMERAL is %d bytes, too small to give up %d bytes and stay usable", sizeBefore, uint64(shrinkBy))
	}

	ephemeralMaxSize := sizeBefore - shrinkBy

	suite.T().Logf("staging wipe of %s on %s", constants.EphemeralPartitionLabel, node)

	suite.Require().NoError(suite.Client.VolumeWipe(nodeCtx, &machineapi.VolumeWipeRequest{
		VolumeIds: []string{constants.EphemeralPartitionLabel},
		OnReboot:  true,
	}))

	onSystemDisk := cel.MustExpression(cel.ParseBooleanExpression("system_disk", celenv.DiskLocator()))

	ephemeralDoc := blockcfg.NewVolumeConfigV1Alpha1()
	ephemeralDoc.MetaName = constants.EphemeralPartitionLabel
	ephemeralDoc.ProvisioningSpec.DiskSelectorSpec.Match = onSystemDisk
	ephemeralDoc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize(strconv.FormatUint(ephemeralMaxSize, 10))

	userVolumeDoc := blockcfg.NewUserVolumeConfigV1Alpha1()
	userVolumeDoc.MetaName = userVolumeName
	userVolumeDoc.ProvisioningSpec.DiskSelectorSpec.Match = onSystemDisk
	userVolumeDoc.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize(strconv.FormatUint(userVolumeMinSize, 10))
	userVolumeDoc.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize(strconv.FormatUint(userVolumeMaxSize, 10))

	suite.T().Cleanup(func() {
		cleanupCtx := client.WithNode(context.Background(), node)

		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, blockcfg.UserVolumeConfigKind, userVolumeName)
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, blockcfg.VolumeConfigKind, constants.EphemeralPartitionLabel)
	})

	suite.T().Logf("staging a config which caps %s and carves out user volume %q", constants.EphemeralPartitionLabel, userVolumeName)

	// Staged rather than applied: the new size is only realizable on the boot that wipes the
	// partition, and applying it live would leave the config and the volume disagreeing until then.
	suite.PatchMachineConfigWithModeSetter(nodeCtx,
		func(req *machineapi.ApplyConfigurationRequest) {
			req.Mode = machineapi.ApplyConfigurationRequest_STAGED
		},
		ephemeralDoc, userVolumeDoc,
	)

	suite.T().Logf("rebooting %s to apply the staged wipe and the staged config", node)

	suite.AssertRebooted(
		suite.ctx, node,
		func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		},
		10*time.Minute,
		suite.CleanupFailedPods,
	)

	rebootedCtx := client.WithNode(suite.ctx, node)

	// EPHEMERAL comes back as a new, smaller partition.
	rtestutils.AssertResource(
		rebootedCtx, suite.T(), suite.Client.COSI,
		constants.EphemeralPartitionLabel,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
			asrt.NotEqual(partitionUUIDBefore, vs.TypedSpec().PartitionUUID, "EPHEMERAL was not re-provisioned")
			asrt.LessOrEqual(vs.TypedSpec().Size, ephemeralMaxSize,
				"EPHEMERAL is %s, above the configured cap", vs.TypedSpec().PrettySize)
		},
	)

	// And the space that freed up is what the user volume is provisioned from.
	rtestutils.AssertResource(
		rebootedCtx, suite.T(), suite.Client.COSI,
		userVolumeID,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
			asrt.Equal(block.VolumeTypePartition, vs.TypedSpec().Type)
		},
	)

	rtestutils.AssertResource(
		rebootedCtx, suite.T(), suite.Client.COSI,
		userVolumeID,
		func(ms *block.MountStatus, asrt *assert.Assertions) {
			asrt.Equal(filepath.Join(constants.UserVolumeMountPoint, userVolumeName), ms.TypedSpec().Target)
		},
	)

	rtestutils.AssertNoResource[*runtimeres.MetaKey](
		rebootedCtx, suite.T(), suite.Client.COSI,
		runtimeres.MetaKeyTagToID(meta.StagedWipeSelectors),
	)
}

func init() {
	allSuites = append(allSuites, new(VolumeWipeSuite))
}
