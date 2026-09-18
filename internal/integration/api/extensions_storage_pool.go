// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// TestStoragePool exercises declarative pools against the real storage daemon.
func (suite *ExtensionsSuiteLibvirt) TestStoragePool() {
	// Allow a reboot in addition to reconciliation; ordinary extension tests have five minutes.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fixture := storagePoolFixture{suite: suite, node: suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)}
	// Unique names ensure cleanup cannot remove pre-existing documents or pool data.
	suffix := uuid.NewString()[:8]
	fixture.poolName = "vm-images-" + suffix
	fixture.volumeNames = []string{"vm-data-" + suffix, "vm-data-other-" + suffix}
	fixture.volumeIDs = []string{constants.UserVolumePrefix + fixture.volumeNames[0], constants.UserVolumePrefix + fixture.volumeNames[1]}
	fixture.paths = make([]string, len(fixture.volumeNames))
	// Separate defers allow restoration to continue even if an earlier assertion fails.
	defer fixture.cleanupVolumes()
	defer fixture.cleanupData()
	defer fixture.cleanupPool()

	nodeCtx := client.WithNode(ctx, fixture.node)
	suite.AssertServicesRunning(ctx, fixture.node, map[string]string{"ext-virtstoraged": "Running"})
	fixture.createVolumes(ctx)

	pool := storagecfg.NewStoragePoolV1Alpha1()
	pool.MetaName = fixture.poolName
	pool.VolumeConfig.VolumeName = constants.UserVolumePrefix + fixture.volumeNames[0]
	suite.PatchMachineConfig(nodeCtx, pool)

	poolUUID := fixture.waitPool(ctx, 0, "")

	// Reapplying an unchanged document must not allocate a new identity.
	suite.PatchMachineConfig(nodeCtx, pool)
	fixture.waitPool(ctx, 0, poolUUID)

	// An empty pool is removed without affecting either backing user volume.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.StoragePoolKind, fixture.poolName)
	fixture.waitAbsent(ctx)
	fixture.assertVolumes(ctx)

	suite.PatchMachineConfig(nodeCtx, pool)
	fixture.waitPool(ctx, 0, poolUUID)
	fixture.writeSentinel(ctx, 0, "original-data")

	// A backing-reference change updates the live path, not the UUID, and does not migrate data.
	pool.VolumeConfig.VolumeName = constants.UserVolumePrefix + fixture.volumeNames[1]
	suite.PatchMachineConfig(nodeCtx, pool)
	fixture.waitPool(ctx, 1, poolUUID)
	fixture.assertSentinel(ctx, 0, "original-data")
	output, exitCode := suite.RunDebugContainer(ctx, fixture.node, "/nix/var/nix/profiles/default/bin/sh", "-c",
		`test -d "$1" && test ! -e "$1/sentinel"`, "check-no-migration", fixture.paths[1])
	suite.Require().Zero(exitCode, "new target must exist without migrated data: %s", output)
	fixture.writeSentinel(ctx, 1, "new-data")

	// Node-level reboot check: this suite runs without Kubernetes.
	suite.AssertRebootedNoChecks(ctx, fixture.node, func(rebootCtx context.Context) error {
		return base.IgnoreGRPCUnavailable(suite.Client.Reboot(rebootCtx))
	}, 5*time.Minute)
	suite.WaitForBootDone(ctx)
	suite.AssertServicesRunning(ctx, fixture.node, map[string]string{"ext-virtstoraged": "Running"})
	fixture.assertVolumes(ctx)
	fixture.waitPool(ctx, 1, poolUUID)

	// Nonempty pools follow the same stop/undefine policy; neither target's data is removed.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.StoragePoolKind, fixture.poolName)
	fixture.waitAbsent(ctx)
	fixture.assertSentinel(ctx, 0, "original-data")
	fixture.assertSentinel(ctx, 1, "new-data")
	fixture.assertVolumes(ctx)
	suite.T().Logf("pool %s kept UUID %s through reapply, retarget and reboot", fixture.poolName, poolUUID)
}

type storagePoolFixture struct {
	suite       *ExtensionsSuiteLibvirt
	node        string
	poolName    string
	volumeNames []string
	volumeIDs   []string
	paths       []string
}

func (f *storagePoolFixture) virsh(ctx context.Context, args ...string) (string, int32) {
	// Do not use the QEMU URI: exercise the dedicated storage socket.
	command := append([]string{"/usr/local/bin/virsh", "--connect", "storage+unix:///system?socket=/run/libvirt/virtstoraged-sock"}, args...)
	output, exitCode := f.suite.RunDebugContainer(ctx, f.node, command...)

	return strings.TrimSpace(output), exitCode
}

func (f *storagePoolFixture) createVolumes(ctx context.Context) {
	nodeCtx := client.WithNode(ctx, f.node)
	for _, name := range f.volumeNames {
		doc := blockcfg.NewUserVolumeConfigV1Alpha1()
		doc.MetaName = name
		doc.VolumeType = new(block.VolumeTypeDirectory)
		f.suite.PatchMachineConfig(nodeCtx, doc)
	}

	f.assertVolumes(ctx)

	for i, id := range f.volumeIDs {
		mount, err := safe.ReaderGetByID[*block.MountStatus](nodeCtx, f.suite.Client.COSI, id)
		f.suite.Require().NoError(err)
		f.paths[i] = filepath.Join(mount.TypedSpec().Target, f.poolName)
	}
}

func (f *storagePoolFixture) assertVolumes(ctx context.Context) {
	nodeCtx := client.WithNode(ctx, f.node)
	rtestutils.AssertResources(nodeCtx, f.suite.T(), f.suite.Client.COSI, f.volumeIDs,
		func(vs *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, vs.TypedSpec().Phase)
			asrt.Equal(block.VolumeTypeDirectory, vs.TypedSpec().Type)
		})
	rtestutils.AssertResources(nodeCtx, f.suite.T(), f.suite.Client.COSI, f.volumeIDs,
		func(ms *block.MountStatus, asrt *assert.Assertions) {
			asrt.NotEmpty(ms.TypedSpec().Target)
		})
}

func (f *storagePoolFixture) waitAbsent(ctx context.Context) {
	nodeCtx := client.WithNode(ctx, f.node)
	rtestutils.AssertNoResource[*storage.StoragePoolSpec](nodeCtx, f.suite.T(), f.suite.Client.COSI, f.poolName)
	rtestutils.AssertNoResource[*storage.StoragePoolStatus](nodeCtx, f.suite.T(), f.suite.Client.COSI, f.poolName)

	// Retry synchronously: RunDebugContainer can call FatalNow on transport errors.
	f.suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		output, exitCode := f.virsh(ctx, "pool-list", "--all", "--name")
		if exitCode != 0 || slices.Contains(strings.Fields(output), f.poolName) {
			return retry.ExpectedErrorf("pool must be stopped and undefined: exit=%d, output=%s", exitCode, output)
		}

		return nil
	}))

	// A failed lookup is meaningful only after a successful list against the same daemon.
	output, exitCode := f.virsh(ctx, "pool-info", f.poolName)
	f.suite.Require().NotZero(exitCode, "removed pool is still defined: %s", output)
}

func (f *storagePoolFixture) waitPool(ctx context.Context, volumeIndex int, expectedUUID string) string {
	nodeCtx := client.WithNode(ctx, f.node)
	rtestutils.AssertResource(nodeCtx, f.suite.T(), f.suite.Client.COSI, f.poolName,
		func(spec *storage.StoragePoolSpec, asrt *assert.Assertions) {
			asrt.Equal(f.volumeIDs[volumeIndex], spec.TypedSpec().VolumeID)
		})
	rtestutils.AssertResource(nodeCtx, f.suite.T(), f.suite.Client.COSI, f.poolName,
		func(status *storage.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.Equal(f.volumeIDs[volumeIndex], status.TypedSpec().VolumeID)
			asrt.Equal(f.paths[volumeIndex], status.TypedSpec().TargetPath)
			asrt.True(status.TypedSpec().Ready)
			asrt.Empty(status.TypedSpec().Error)
		})

	var livePool libvirtxml.StoragePool

	// Verify the live libvirt pool, not just COSI readiness.
	f.suite.Require().NoError(retry.Constant(time.Minute, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		output, exitCode := f.virsh(ctx, "pool-dumpxml", f.poolName)
		if exitCode != 0 {
			return retry.ExpectedErrorf("pool-dumpxml: exit=%d, output=%s", exitCode, output)
		}

		livePool = libvirtxml.StoragePool{}
		if err := livePool.Unmarshal(output); err != nil {
			return err
		}

		if livePool.Target == nil {
			return retry.ExpectedErrorf("pool target is missing")
		}

		if livePool.Target.Path != f.paths[volumeIndex] {
			return retry.ExpectedErrorf("pool target is still %q, want %q", livePool.Target.Path, f.paths[volumeIndex])
		}

		output, exitCode = f.virsh(ctx, "pool-list", "--name")
		if exitCode != 0 || !slices.Contains(strings.Fields(output), f.poolName) {
			return retry.ExpectedErrorf("pool must be active: exit=%d, output=%s", exitCode, output)
		}

		return nil
	}))
	f.suite.Require().Equal("dir", livePool.Type)
	f.suite.Require().Equal(f.poolName, livePool.Name)
	parsedUUID, err := uuid.Parse(livePool.UUID)
	f.suite.Require().NoError(err)
	f.suite.Require().NotEqual(uuid.Nil, parsedUUID)

	if expectedUUID != "" {
		f.suite.Require().Equal(expectedUUID, livePool.UUID)
	}

	output, exitCode := f.virsh(ctx, "pool-info", f.poolName)
	f.suite.Require().Zero(exitCode, "pool-info: %s", output)
	f.suite.Require().Regexp(`(?m)^State:\s+running\s*$`, output)
	f.suite.Require().Regexp(`(?m)^Persistent:\s+yes\s*$`, output)
	f.suite.Require().Regexp(`(?m)^Autostart:\s+no\s*$`, output)

	return livePool.UUID
}

func (f *storagePoolFixture) writeSentinel(ctx context.Context, volumeIndex int, contents string) {
	output, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/sh", "-c",
		`printf '%s' "$2" > "$1/sentinel"`, "write-sentinel", f.paths[volumeIndex], contents)
	f.suite.Require().Zero(exitCode, "write sentinel: %s", output)
}

func (f *storagePoolFixture) assertSentinel(ctx context.Context, volumeIndex int, contents string) {
	output, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/busybox", "cat", filepath.Join(f.paths[volumeIndex], "sentinel"))
	f.suite.Require().Zero(exitCode, "read sentinel: %s", output)
	f.suite.Require().Equal(contents, output, "sentinel was lost or changed")
}

func (f *storagePoolFixture) cleanupVolumes() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	f.suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, f.node), blockcfg.UserVolumeConfigKind, f.volumeNames...)
}

func (f *storagePoolFixture) cleanupData() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	for _, path := range f.paths {
		if path == "" {
			continue
		}

		// Remove only our sentinel and empty pool directory, never recursively remove a volume.
		output, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/sh", "-c",
			`/nix/var/nix/profiles/default/bin/busybox rm -f "$1/sentinel"
if test -d "$1"; then /nix/var/nix/profiles/default/bin/busybox rmdir "$1"; fi`, "cleanup", path)
		f.suite.Assert().Zero(exitCode, "pool data cleanup: %s", output)
	}
}

func (f *storagePoolFixture) cleanupPool() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// A broken reconciler must not leave a test-owned libvirt definition behind.
	defer f.cleanupDefinition()

	f.suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, f.node), storagecfg.StoragePoolKind, f.poolName)
	f.waitAbsent(ctx)
}

func (f *storagePoolFixture) cleanupDefinition() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	output, exitCode := f.virsh(ctx, "pool-list", "--all", "--name")
	if !f.suite.Assert().Zero(exitCode, "list pools for cleanup: %s", output) || !slices.Contains(strings.Fields(output), f.poolName) {
		return
	}

	output, exitCode = f.virsh(ctx, "pool-destroy", f.poolName)
	if exitCode != 0 {
		f.suite.T().Logf("cleanup pool-destroy (pool may already be inactive): %s", output)
	}

	output, exitCode = f.virsh(ctx, "pool-undefine", f.poolName)
	f.suite.Assert().Zero(exitCode, "undefine test pool during cleanup: %s", output)
}
