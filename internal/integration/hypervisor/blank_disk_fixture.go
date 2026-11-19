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

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeID,
		func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
		})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeID,
		func(status *block.MountStatus, asrt *assert.Assertions) {
			asrt.NotEmpty(status.TypedSpec().Target)
		})

	mount, err := safe.ReaderGetByID[*block.MountStatus](nodeCtx, suite.Client.State(), fixture.volumeID)
	suite.Require().NoError(err)

	fixture.poolPath = filepath.Join(mount.TypedSpec().Target, fixture.poolName)

	pool := storagecfg.NewStoragePoolConfigV1Alpha1()
	pool.MetaName = fixture.poolName
	pool.VolumeConfig.VolumeName = fixture.volumeName
	suite.PatchMachineConfig(nodeCtx, pool)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), fixture.poolName,
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
	doc.FirmwareConfig.FirmwareType = f.suite.guestArch(f.node).firmware
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
		removeBlankDiskVolume(nodeCtx, f.suite.T(), f.suite.Client.State(), f.volumeStatusID(), func() {
			output, exitCode := f.suite.RunDebugContainer(ctx, f.node, "/nix/var/nix/profiles/default/bin/busybox",
				"rm", "-f", filepath.Join(f.poolPath, f.volumeFile))
			f.suite.Assert().Zero(exitCode, "remove test volume: %s", output)
		})
	}

	f.suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.StoragePoolConfigKind, f.poolName)
	f.suite.RemoveMachineConfigDocumentsByName(nodeCtx, blockcfg.UserVolumeConfigKind, f.volumeName)
}

func removeBlankDiskVolume(ctx context.Context, t *testing.T, st state.State, id resource.ID, remove func()) {
	t.Helper()

	rtestutils.AssertNoResource[*storage.StoragePoolVolumeSpec](ctx, t, st, id)
	rtestutils.AssertNoResource[*storage.StoragePoolVolumeStatus](ctx, t, st, id)
	remove()
}

func (f *blankDiskFixture) writableImageDocument(library string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := f.document("2GiB", hypervisorhelpers.PowerStateRunning)
	doc.MemoryConfig.MemorySize = meta.MustByteSize("256MiB")
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.DisksConfig[0].ProvisionConfig = hypervisorcfg.VirtualMachineDiskProvision{
		FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
			ImageLibrary: library,
			ImageFile:    "guest.img",
		},
	}

	return doc
}
