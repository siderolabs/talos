// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/client"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// blankDiskFixture is one virtual machine with one blank disk, in a pool of its own.
type blankDiskFixture struct {
	suite *LibvirtSuite

	node       string
	volumeName string
	volumeID   string
	poolName   string
	vmName     string
	diskName   string
	volumeFile string
	poolPath   string
}

// newBlankDiskFixture declares the backing volume and the pool, and waits for both.
//
// Every name carries a unique suffix: these tests write into a pool which, by design, nothing ever
// cleans up on their behalf, so they must not be able to collide with each other or with anything
// already on the node.
func newBlankDiskFixture(ctx context.Context, suite *LibvirtSuite, kind string) *blankDiskFixture {
	suite.T().Helper()

	return newBlankDiskFixtureOnNode(ctx, suite, kind, suite.RandomDiscoveredNodeInternalIP())
}

func newBlankDiskFixtureOnNode(ctx context.Context, suite *LibvirtSuite, kind, node string) *blankDiskFixture {
	suite.T().Helper()

	suffix := uuid.NewString()[:8]
	fixture := &blankDiskFixture{
		suite:      suite,
		node:       node,
		volumeName: "vm-blank-" + suffix,
		poolName:   "vm-pool-" + suffix,
		vmName:     kind + "-" + suffix,
		diskName:   "data",
	}
	fixture.volumeID = constants.UserVolumePrefix + fixture.volumeName
	fixture.volumeFile = fixture.vmName + "__" + fixture.diskName + ".qcow2"

	nodeCtx := client.WithNode(ctx, fixture.node)

	suite.AssertServicesRunning(ctx, fixture.node, map[string]string{
		"ext-virtstoraged": "Running",
		"ext-virtqemud":    "Running",
	})

	// Registered before anything is applied: a failed apply still leaves state behind.
	suite.T().Cleanup(fixture.cleanup)

	volume := blockcfg.NewUserVolumeConfigV1Alpha1()
	volume.MetaName = fixture.volumeName
	volume.VolumeType = new(block.VolumeTypeDirectory)
	suite.PatchMachineConfig(nodeCtx, volume)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeID,
		func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
		})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeID,
		func(status *block.MountStatus, asrt *assert.Assertions) {
			asrt.NotEmpty(status.TypedSpec().Target)
		})

	mount, err := safe.ReaderGetByID[*block.MountStatus](nodeCtx, suite.Client.COSI, fixture.volumeID)
	suite.Require().NoError(err)

	fixture.poolPath = filepath.Join(mount.TypedSpec().Target, fixture.poolName)

	pool := storagecfg.NewStoragePoolV1Alpha1()
	pool.MetaName = fixture.poolName
	pool.VolumeConfig.VolumeName = fixture.volumeName
	suite.PatchMachineConfig(nodeCtx, pool)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.poolName,
		func(status *storage.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.Equal(storage.StoragePoolPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)
			asrt.Equal(fixture.poolPath, status.TypedSpec().TargetPath)
		})

	return fixture
}

// volumeID is the ID the disk's volume is tracked under.
func (f *blankDiskFixture) volumeStatusID() string {
	return storage.StoragePoolVolumeID(f.poolName, f.volumeFile)
}

// diskStatusID is the ID the disk's status is published under. Unlike the image-provisioned helper,
// the digest covers the pool and the format, and deliberately not the size.
func (f *blankDiskFixture) diskStatusID() string {
	return hypervisor.VirtualMachineDiskStatusID(f.vmName, hypervisor.VirtualMachineDiskSpec{
		Name:      f.diskName,
		Pool:      f.poolName,
		Format:    "qcow2",
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
	})
}

// document builds the virtual machine, with its disk at the given size.
func (f *blankDiskFixture) document(size string, powerState hypervisorhelpers.PowerState) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = f.vmName
	doc.PowerStateConfig = powerState
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:   f.diskName,
			DiskPool:   f.poolName,
			DiskSize:   meta.MustByteSize(size),
			DiskFormat: hypervisorhelpers.VirtualMachineDiskFormatQCOW2,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
			},
		},
	}

	return doc
}

// virtualSize reads the volume's capacity out of its qcow2 header, so the status is checked against
// what QEMU would see rather than against the controller's own account of it.
//
// The header is read directly rather than through qemu-img: that binary belongs to the libvirt
// extension and its path is not this suite's to assume, while busybox is already how every other
// check here reaches the node. A qcow2 header is a magic and a version, then the backing file
// fields, then the size at offset 24 as a big-endian uint64.
func (f *blankDiskFixture) virtualSize(ctx context.Context) uint64 {
	f.suite.T().Helper()

	path := filepath.Join(f.poolPath, f.volumeFile)

	magic := f.readHeaderBytes(ctx, path, 0, 4)
	f.suite.Require().Equal("514649fb", magic, "volume is not a qcow2 file")

	hex := f.readHeaderBytes(ctx, path, 24, 8)

	size, err := strconv.ParseUint(hex, 16, 64)
	f.suite.Require().NoError(err, "qcow2 size field %q", hex)

	return size
}

// readHeaderBytes returns count bytes of a file from offset, as lowercase hex.
func (f *blankDiskFixture) readHeaderBytes(ctx context.Context, path string, offset, count int) string {
	f.suite.T().Helper()

	const busybox = "/nix/var/nix/profiles/default/bin/busybox"

	output, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/sh", "-c",
		busybox+` dd if="$1" bs=1 skip="$2" count="$3" 2>/dev/null | `+busybox+` od -An -tx1 | `+busybox+` tr -d ' \n'`,
		"read-header", path, strconv.Itoa(offset), strconv.Itoa(count))
	f.suite.Require().Zero(exitCode, "read %d bytes at %d: %s", count, offset, output)

	return output
}

// fileExists reports whether the volume's file is still on the host.
func (f *blankDiskFixture) fileExists(ctx context.Context) bool {
	_, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/busybox",
		"test", "-f", filepath.Join(f.poolPath, f.volumeFile))

	return exitCode == 0
}

// cleanup removes everything the fixture declared, and then the volume itself: nothing in Talos
// deletes a pool volume, which is the point of the feature and the reason this has to.
func (f *blankDiskFixture) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	nodeCtx := client.WithNode(ctx, f.node)

	f.suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, f.vmName)
	f.suite.assertNoDomainWithContext(ctx, f.node, f.vmName)

	if f.poolPath != "" {
		removeBlankDiskVolume(nodeCtx, f.suite.T(), f.suite.Client.COSI, f.volumeStatusID(), func() {
			output, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/busybox",
				"rm", "-f", filepath.Join(f.poolPath, f.volumeFile))
			f.suite.Assert().Zero(exitCode, "remove test volume: %s", output)
		})
	}

	f.suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.StoragePoolKind, f.poolName)
	f.suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, f.volumeName)
}

func removeBlankDiskVolume(ctx context.Context, t *testing.T, st state.State, id resource.ID, remove func()) {
	t.Helper()

	rtestutils.AssertNoResource[*storage.StoragePoolVolumeSpec](ctx, t, st, id)
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeStatus](ctx, t, st, id)
	remove()
}

// TestBlankDiskProvisioning covers the whole life of a blank disk: created, attached writable to a
// running guest, grown, refused a shrink, retained when its configuration goes, and adopted back.
func (suite *LibvirtSuite) TestBlankDiskProvisioning() {
	suite.requireContentLibrary()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fixture := newBlankDiskFixture(ctx, suite, "vm-blank")
	nodeCtx := client.WithNode(ctx, fixture.node)

	suite.PatchMachineConfig(nodeCtx, fixture.document("64MiB", hypervisorhelpers.PowerStateRunning))

	source := filepath.Join(fixture.poolPath, fixture.volumeFile)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID(),
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

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, diskID,
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, "error: %q", status.TypedSpec().Error)
			asrt.Equal(source, status.TypedSpec().SourcePath)
			asrt.Equal("qcow2", status.TypedSpec().Format)
			asrt.False(status.TypedSpec().ReadOnly, "a blank disk is the guest's to write into")
			asrt.True(status.TypedSpec().Blank)
			asrt.Equal(fixture.poolName, status.TypedSpec().Pool)
			asrt.Equal(fixture.volumeFile, status.TypedSpec().Volume)
		})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.vmName,
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

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID(),
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

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID(),
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

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(uint64(128<<20), status.TypedSpec().Capacity)
			asrt.Contains(status.TypedSpec().Error, "never shrunk")
		})

	suite.Require().Equal(uint64(128<<20), fixture.virtualSize(ctx))

	// Removing the virtual machine withdraws everything that tracked the volume, and nothing else.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, fixture.vmName)

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.COSI, diskID)
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeSpec](nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID())
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeStatus](nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID())

	suite.Require().True(fixture.fileExists(ctx), "removing the configuration must never delete a volume")
	suite.Require().Equal(uint64(128<<20), fixture.virtualSize(ctx))

	// The same names mean the same disk: the volume comes back at the size it was left at, not at
	// the size asked for, and is not recreated.
	suite.PatchMachineConfig(nodeCtx, fixture.document("64MiB", hypervisorhelpers.PowerStateStopped))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(uint64(128<<20), status.TypedSpec().Capacity, "an adopted volume keeps what it held")
		})
}

// TestBlankDiskPoolRemovalStopsGuest withdraws a running guest and its pool in one valid config.
// The controllers must release the guest's disk hold so the pool can be undefined without
// deleting the volume's file.
func (suite *LibvirtSuite) TestBlankDiskPoolRemovalStopsGuest() {
	suite.requireContentLibrarySupport()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	fixture := newBlankDiskFixture(ctx, suite, "vm-poolgone")
	nodeCtx := client.WithNode(ctx, fixture.node)

	suite.PatchMachineConfig(nodeCtx, fixture.document("64MiB", hypervisorhelpers.PowerStateRunning))

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID(),
		func(status *storage.StoragePoolVolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == storage.StoragePoolVolumePhaseReady, "error: %q", status.TypedSpec().Error)
		})

	suite.assertRunningTransientDomainWithDevices(fixture.node, fixture.vmName, 1, 1, 0)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, fixture.poolName,
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
			"kind":       storagecfg.StoragePoolKind,
			"name":       fixture.poolName,
			"$patch":     "delete",
		},
	)

	// These eventual absences establish convergence, not the exact order of teardown events.
	suite.assertNoDomain(fixture.node, fixture.vmName)
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeStatus](nodeCtx, suite.T(), suite.Client.COSI, fixture.volumeStatusID())
	rtestutils.AssertNoResource[*storage.StoragePoolSpec](nodeCtx, suite.T(), suite.Client.COSI, fixture.poolName)
	rtestutils.AssertNoResource[*storage.StoragePoolStatus](nodeCtx, suite.T(), suite.Client.COSI, fixture.poolName)

	suite.Require().True(fixture.fileExists(ctx), "a pool's contents outlive the pool")
}
