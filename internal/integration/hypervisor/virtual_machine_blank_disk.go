// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// BlankDiskSuite verifies blank disk provisioning.
type BlankDiskSuite struct{ LibvirtSuite }

// SuiteName implements base.NamedSuite.
func (suite *BlankDiskSuite) SuiteName() string { return "hypervisor.BlankDiskSuite" }

// TestBlankDiskProvisioning covers the whole life of a blank disk: created, attached writable to a
// running guest, grown, refused a shrink, retained when its configuration goes, and adopted back.
func (suite *BlankDiskSuite) TestBlankDiskProvisioning() {
	suite.requireContentLibrary()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fixture := newBlankDiskFixture(ctx, &suite.LibvirtSuite, "vm-blank")
	nodeCtx := client.WithNode(ctx, fixture.node)

	suite.PatchMachineConfig(nodeCtx, fixture.document("64MiB", hypervisorhelpers.PowerStateRunning))

	source := filepath.Join(fixture.poolPath, fixture.volumeFile)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(source, status.TypedSpec().Path)
			asrt.Equal("qcow2", status.TypedSpec().Format)
			asrt.Equal(uint64(64<<20), status.TypedSpec().Capacity)
			asrt.Zero(status.TypedSpec().PendingCapacity)
		})

	// Checked against the file rather than the status: the controller's account of the volume and
	// what QEMU would open are two different things.
	suite.Require().Equal(uint64(64<<20), fixture.virtualSize(ctx))

	diskID := fixture.diskStatusID()

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), diskID,
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)
			asrt.Equal(source, status.TypedSpec().SourcePath)
			asrt.Equal("qcow2", status.TypedSpec().Format)
			asrt.False(status.TypedSpec().ReadOnly, "a blank disk is the guest's to write into")
			asrt.True(status.TypedSpec().Blank)
			asrt.Equal(fixture.poolName, status.TypedSpec().Pool)
			asrt.Equal(fixture.volumeFile, status.TypedSpec().Volume)
		})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.vmName,
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, `device="disk"`)
			asrt.Contains(spec.TypedSpec().DomainXML, `<source file="`+source+`">`)
			asrt.NotContains(spec.TypedSpec().DomainXML, `<readonly>`)
		})

	// The first guest ever to boot off a writable pool volume: this is what proves the SELinux
	// relabel rules, which are fatal to domain start when missing rather than merely denied.
	suite.assertRunningTransientDomainWithDevices(fixture.node, fixture.vmName, 1, 1, 0)

	// A growth asked for while the guest has the volume open is remembered, not forced: for qcow2
	// QEMU holds a write lock, and for raw the resize would succeed while the guest saw the old size.
	suite.PatchMachineConfig(nodeCtx, fixture.document("128MiB", hypervisorhelpers.PowerStateRunning))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(storage.StoragePoolVolumePhaseReady, status.TypedSpec().Phase)
			asrt.Equal(uint64(64<<20), status.TypedSpec().Capacity)
			asrt.Equal(uint64(128<<20), status.TypedSpec().PendingCapacity)
			asrt.Contains(status.TypedSpec().Error, "detach all consumers to apply growth")
		})

	suite.Require().Equal(uint64(64<<20), fixture.virtualSize(ctx))

	// The size is deliberately absent from the disk status ID, so a resize updates the status a
	// running domain already holds rather than minting a second one and restarting the guest.
	suite.Require().Equal(diskID, fixture.diskStatusID())
	suite.assertRunningTransientDomainWithDevices(fixture.node, fixture.vmName, 1, 1, 0)

	// Stopping the guest lets the growth through.
	suite.PatchMachineConfig(nodeCtx, fixture.document("128MiB", hypervisorhelpers.PowerStateStopped))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(uint64(128<<20), status.TypedSpec().Capacity)
			asrt.Zero(status.TypedSpec().PendingCapacity)
			asrt.Empty(status.TypedSpec().Error)
		})

	suite.Require().Equal(uint64(128<<20), fixture.virtualSize(ctx))

	// A smaller size is reported and ignored. The disk stays attachable: stopping a guest over an
	// edited number is a worse outcome than the mismatch.
	suite.PatchMachineConfig(nodeCtx, fixture.document("32MiB", hypervisorhelpers.PowerStateStopped))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(uint64(128<<20), status.TypedSpec().Capacity)
			asrt.Contains(status.TypedSpec().Error, "never shrunk")
		})

	suite.Require().Equal(uint64(128<<20), fixture.virtualSize(ctx))

	// Removing the virtual machine withdraws everything that tracked the volume, and nothing else.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, fixture.vmName)

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.State(), diskID)
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeSpec](nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID())
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeStatus](nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID())

	suite.Require().True(fixture.fileExists(ctx), "removing the configuration must never delete a volume")
	suite.Require().Equal(uint64(128<<20), fixture.virtualSize(ctx))

	// The same names mean the same disk: the volume comes back at the size it was left at, not at
	// the size asked for, and is not recreated.
	suite.PatchMachineConfig(nodeCtx, fixture.document("64MiB", hypervisorhelpers.PowerStateStopped))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(uint64(128<<20), status.TypedSpec().Capacity, "an adopted volume keeps what it held")
		})
}

// TestBlankDiskPoolRemovalStopsGuest withdraws a running guest and its pool in one valid config.
// The controllers must release the guest's disk hold so the pool can be undefined without
// deleting the volume's file.
func (suite *BlankDiskSuite) TestBlankDiskPoolRemovalStopsGuest() {
	suite.requireContentLibrarySupport()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fixture := newBlankDiskFixture(ctx, &suite.LibvirtSuite, "vm-poolgone")
	nodeCtx := client.WithNode(ctx, fixture.node)

	suite.PatchMachineConfig(nodeCtx, fixture.document("64MiB", hypervisorhelpers.PowerStateRunning))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
		})

	suite.assertRunningTransientDomainWithDevices(fixture.node, fixture.vmName, 1, 1, 0)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.poolName,
		func(status *storage.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.True(status.Metadata().Finalizers().Has("storage.StoragePoolVolumeController"), "the attached volume must hold the pool")
		})

	// Removing only the referenced pool is invalid. Withdraw both desired documents in one
	// configuration while the guest is still running; the controllers arbitrate teardown.
	suite.PatchMachineConfig(nodeCtx,
		map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       hypervisorcfg.VirtualMachineConfigKind,
			"name":       fixture.vmName,
			"$patch":     "delete",
		},
		map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       storagecfg.StoragePoolConfigKind,
			"name":       fixture.poolName,
			"$patch":     "delete",
		},
	)

	// These eventual absences establish convergence, not the exact order of teardown events.
	suite.assertNoDomain(fixture.node, fixture.vmName)
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeStatus](nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID())
	rtestutils.AssertNoResource[*storage.StoragePoolSpec](nodeCtx, suite.T(), suite.Client.State(), fixture.poolName)
	rtestutils.AssertNoResource[*storage.StoragePoolStatus](nodeCtx, suite.T(), suite.Client.State(), fixture.poolName)

	suite.Require().True(fixture.fileExists(ctx), "a pool's contents outlive the pool")
}

func init() {
	allSuites = append(allSuites, new(BlankDiskSuite))
}
