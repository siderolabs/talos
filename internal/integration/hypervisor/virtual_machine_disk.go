// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	storagecfg "github.com/siderolabs/talos/pkg/machinery/config/types/storage"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// isoName is the library file the cdrom tests attach. Its contents are not a real ISO: nothing
// boots it, and the domain only has to name it.
const isoName = "install.iso"

// contentLibraryControllerName mirrors the controller whose mount requests are keyed by its name.
const contentLibraryControllerName = "hypervisor.ContentLibraryController"

// requireContentLibrarySupport is requireContentLibrary for the cases kept out of the short pipeline.
func (suite *LibvirtSuite) requireContentLibrarySupport() {
	suite.T().Helper()

	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

	suite.requireContentLibrary()
}

// requireContentLibrary skips unless the cluster can hold the library these tests source their
// image from. LibvirtSuite itself only requires the hypervisor.
func (suite *LibvirtSuite) requireContentLibrary() {
	suite.T().Helper()

	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}
}

// TestCDROMFromContentLibrary attaches an independent pool copy of a library object.
func (suite *LibvirtSuite) TestCDROMFromContentLibrary() {
	suite.requireContentLibrarySupport()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	pool, poolPath := suite.provisionContentLibraryPool(nodeCtx, node, library)

	contents := []byte("talos integration cdrom")
	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, "", bytes.NewReader(contents))
	suite.Require().NoError(err)

	name := "vm-cdrom-" + uuid.NewString()

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	installDisk := hypervisorcfg.VirtualMachineDisk{
		DiskName:      "install",
		DiskPool:      pool,
		DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
		DiskBootOrder: 1,
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
			FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
				ImageLibrary: library,
				ImageFile:    isoName,
				// Pinned, so the node is made to hash the file rather than take it on trust.
				ImageDigest: digest.FromBytes(contents).String(),
			},
		},
	}
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{installDisk}

	// Register cleanup before applying configuration: a failed apply can still leave a guest
	// behind. Do not reuse the test's expiring context.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	var source string

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, installDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
			source = status.TypedSpec().SourcePath
			asrt.Equal(filepath.Join(poolPath, status.TypedSpec().Volume), source)
			asrt.NotContains(source, libraryPath+"/objects/")
			asrt.Equal("raw", status.TypedSpec().Format)
			asrt.True(status.TypedSpec().ReadOnly)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), name,
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, `device="cdrom"`)
			asrt.Contains(spec.TypedSpec().DomainXML, `<source file="`+source+`">`)
			asrt.Contains(spec.TypedSpec().DomainXML, `<readonly>`)
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	// Removing the disk takes its status with it, and leaves the guest running without it.
	//
	// Deleted with a selector rather than by reapplying the document without it: disks merge by
	// name, and an empty list is omitted from a patch altogether, so neither expresses a removal.
	doc.DisksConfig = nil

	suite.PatchMachineConfig(nodeCtx, map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       hypervisorcfg.VirtualMachineConfigKind,
		"name":       name,
		"disks": []any{
			map[string]any{"name": "install", "$patch": "delete"},
		},
	})

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.State(),
		diskStatusID(name, installDisk))
	suite.assertRunningTransientDomain(node, name, 1)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	suite.assertNoDomain(node, name)
}

// TestCDROMWaitsForItsImage covers a cdrom naming a file the library does not hold: the disk says
// why, and no domain is defined until the image shows up.
func (suite *LibvirtSuite) TestCDROMWaitsForItsImage() {
	suite.requireContentLibrarySupport()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, _ := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	pool, _ := suite.provisionContentLibraryPool(nodeCtx, node, library)

	name := "vm-absent-" + uuid.NewString()

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName: "install",
			DiskPool: pool,
			DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: library,
					ImageFile:    isoName,
				},
			},
		},
	}

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.False(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady)
			asrt.Contains(status.TypedSpec().Error, isoName)
		},
	)

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.State(), name)
	suite.assertNoDomain(node, name)

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, "", bytes.NewReader([]byte("late arrival")))
	suite.Require().NoError(err)

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)
}

// TestCDROMFromTalosISO covers the same path as TestCDROMFromContentLibrary, but with a real Talos
// ISO rather than a few bytes standing in for one: a hundreds-of-megabytes image uploaded through
// the streaming API, hashed on the way in, hashed again when the disk resolves, and opened by QEMU.
//
// It asserts provisioning only. Nothing here waits for the guest to boot: observing that needs a
// serial console history or guest networking, neither of which exists yet.
func (suite *LibvirtSuite) TestCDROMFromTalosISO() {
	suite.requireContentLibrarySupport()

	if suite.HypervisorTalosISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.talos-iso is not set")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	pool, poolPath := suite.provisionContentLibraryPool(nodeCtx, node, library)

	info, err := os.Stat(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	dgst := suite.digestOfISO()

	suite.T().Logf("uploading %s (%d bytes, %s) into content library %q", suite.HypervisorTalosISOPath, info.Size(), dgst, library)

	iso, err := os.Open(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	defer iso.Close() //nolint:errcheck

	started := time.Now()

	// The digest is declared, so the node verifies what it received before committing it.
	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, dgst.String(), iso)
	suite.Require().NoError(err)

	suite.T().Logf("upload took %s", time.Since(started).Round(time.Millisecond))

	name := "vm-talos-iso-" + uuid.NewString()

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("512MiB")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "install",
			DiskPool:      pool,
			DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: library,
					ImageFile:    isoName,
					// Pinned on a real image: this is the hash the disk controller re-runs over
					// hundreds of megabytes on every resolution.
					ImageDigest: dgst.String(),
				},
			},
		},
	}

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(filepath.Join(poolPath, status.TypedSpec().Volume), status.TypedSpec().SourcePath)
			asrt.NotEqual(contentLibraryObjectPath(libraryPath, dgst), status.TypedSpec().SourcePath)
		},
	)

	// libvirt refuses to start a domain whose backing file it cannot open, so a running domain is
	// what proves QEMU reached the pool copy and its labels.
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	// QEMU opens only the independent pool copy.
	blocks := suite.runVirsh(node, "domblklist", name, "--details")
	suite.Require().Contains(blocks, poolPath)
	suite.Require().NotContains(blocks, contentLibraryObjectPath(libraryPath, dgst))

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	suite.assertNoDomain(node, name)
}

// digestOfISO hashes the ISO in one streaming pass, so a large image never lands in memory.
func (suite *LibvirtSuite) digestOfISO() digest.Digest {
	suite.T().Helper()

	f, err := os.Open(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	defer f.Close() //nolint:errcheck

	digester := digest.SHA256.Digester()

	_, err = io.Copy(digester.Hash(), f)
	suite.Require().NoError(err)

	return digester.Digest()
}

// TestCDROMImageIsHeldWhileAttached checks pool-copy lifetime, alias independence and identity changes.
func (suite *LibvirtSuite) TestCDROMImageIsHeldWhileAttached() {
	suite.requireContentLibrarySupport()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	pool, poolPath := suite.provisionContentLibraryPool(nodeCtx, node, library)

	const replacementISO = "replacement.iso"

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, "", bytes.NewReader([]byte("talos first cdrom")))
	suite.Require().NoError(err)

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, replacementISO, "", bytes.NewReader([]byte("talos second cdrom")))
	suite.Require().NoError(err)

	name := "vm-swap-" + uuid.NewString()

	installDisk := hypervisorcfg.VirtualMachineDisk{
		DiskName:      "install",
		DiskPool:      pool,
		DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
		DiskBootOrder: 1,
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
			FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
				ImageLibrary: library,
				ImageFile:    isoName,
			},
		},
	}

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{installDisk}

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, installDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(filepath.Join(poolPath, status.TypedSpec().Volume), status.TypedSpec().SourcePath)
			asrt.NotContains(status.TypedSpec().SourcePath, libraryPath+"/objects/")
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	// Existing aliases remain create-only, but deleting one cannot invalidate a pool copy.
	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, "", bytes.NewReader([]byte("talos overwritten")))
	suite.Require().Equal(codes.AlreadyExists, grpcstatus.Code(err))
	_, err = suite.Client.ContentLibraryService().Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Target:    &machineapi.ContentLibraryServiceDeleteRequest_Name{Name: isoName},
	})
	suite.Require().NoError(err)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)
	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, "", bytes.NewReader([]byte("talos overwritten")))
	suite.Require().NoError(err)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	swappedDisk := installDisk
	swappedDisk.ProvisionConfig.FromImageConfig = &hypervisorcfg.VirtualMachineDiskFromImage{
		ImageLibrary: library,
		ImageFile:    replacementISO,
	}

	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{swappedDisk}
	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, swappedDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
			asrt.Equal(filepath.Join(poolPath, status.TypedSpec().Volume), status.TypedSpec().SourcePath)
			asrt.NotContains(status.TypedSpec().SourcePath, libraryPath+"/objects/")
		},
	)

	// The status of the previous image is a resource of its own, and it goes once the domain has
	// been redefined without it.
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.State(),
		diskStatusID(name, installDisk))

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	blocks := suite.runVirsh(node, "domblklist", name, "--details")
	suite.Require().Contains(blocks, poolPath)
	suite.Require().NotContains(blocks, libraryPath+"/objects/")

	// Nothing reads the first image any more, so it may go.
	_, err = suite.Client.ContentLibraryService().Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Target:    &machineapi.ContentLibraryServiceDeleteRequest_Name{Name: isoName},
	})
	suite.Require().NoError(err)

	// Stopping releases the name hold, but does not permit overwrite. Reuse requires an explicit delete.
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertNoDomain(node, name)

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, replacementISO, "", bytes.NewReader([]byte("talos replaced")))
	suite.Require().Equal(codes.AlreadyExists, grpcstatus.Code(err))
	_, err = suite.Client.ContentLibraryService().Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Target:    &machineapi.ContentLibraryServiceDeleteRequest_Name{Name: replacementISO},
	})
	suite.Require().NoError(err)
	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, replacementISO, "", bytes.NewReader([]byte("talos replaced")))
	suite.Require().NoError(err)

	// Which is the other half of the bargain: an image the operator may replace is one a guest must
	// not be handed on the strength of what it hashed to before. Pinning what the file used to be
	// now names contents the library no longer holds.
	pinnedDisk := swappedDisk
	pinnedDisk.ProvisionConfig.FromImageConfig = &hypervisorcfg.VirtualMachineDiskFromImage{
		ImageLibrary: library,
		ImageFile:    replacementISO,
		ImageDigest:  digest.FromBytes([]byte("talos second cdrom")).String(),
	}

	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{pinnedDisk}
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, pinnedDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.False(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady)
			asrt.Contains(status.TypedSpec().Error, "digest mismatch")
		},
	)

	// No guest comes up on bytes nothing vouches for.
	suite.assertNoDomain(node, name)

	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{swappedDisk}
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, doc)

	_, err = suite.Client.ContentLibraryService().Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Target:    &machineapi.ContentLibraryServiceDeleteRequest_Name{Name: replacementISO},
	})
	suite.Require().NoError(err)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	suite.assertNoDomain(node, name)
}

// TestContentLibraryIsReleasedWithItsGuest covers the far end of the same chain: a guest reading an
// image out of a library's mount holds the disk status, the library status and the mount underneath
// both, and every one of those has to come back when the guest goes. A hold left behind pins the
// mount until the node reboots.
//
// The library cannot be dropped from the configuration while a virtual machine names it — validation
// refuses that apply — so the guest goes first, and the library after it. Nothing here needs a
// bootable image, so it runs in the short pipeline too.
func (suite *LibvirtSuite) TestContentLibraryIsReleasedWithItsGuest() {
	suite.requireContentLibrary()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	pool, poolPath := suite.provisionContentLibraryPool(nodeCtx, node, library)

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, "", bytes.NewReader([]byte("talos held cdrom")))
	suite.Require().NoError(err)

	name := "vm-release-" + uuid.NewString()

	installDisk := hypervisorcfg.VirtualMachineDisk{
		DiskName:      "install",
		DiskPool:      pool,
		DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
		DiskBootOrder: 1,
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
			FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
				ImageLibrary: library,
				ImageFile:    isoName,
			},
		},
	}

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{installDisk}

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	var source string

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{diskStatusID(name, installDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "error: %q", status.TypedSpec().Error)
			source = status.TypedSpec().SourcePath
			asrt.Equal(filepath.Join(poolPath, status.TypedSpec().Volume), source)
			asrt.NotContains(source, libraryPath+"/objects/")
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)
	suite.Require().Contains(suite.runVirsh(node, "domblklist", name, "--details"), source)

	// Only the pool copy remains attached. The source library has no consumer holds.
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{library},
		func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
			asrt.True(status.Metadata().Finalizers().Empty(), "a completed copy must release its library")
		},
	)

	mountRequestID := contentLibraryControllerName + "/" + library + "/" + constants.UserVolumePrefix + library

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.State(), []string{mountRequestID},
		func(*block.VolumeMountStatus, *assert.Assertions) {},
	)

	// The guest goes, and every hold comes back with it. The library is only removable once no
	// virtual machine names it any more.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	suite.assertNoDomain(node, name)

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.State(),
		diskStatusID(name, installDisk))

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.ContentLibraryConfigKind, library)

	rtestutils.AssertNoResource[*hypervisor.ContentLibraryStatus](nodeCtx, suite.T(), suite.Client.State(), library)
	rtestutils.AssertNoResource[*block.VolumeMountRequest](nodeCtx, suite.T(), suite.Client.State(), mountRequestID)
	rtestutils.AssertNoResource[*block.VolumeMountStatus](nodeCtx, suite.T(), suite.Client.State(), mountRequestID)
}

// provisionContentLibraryPool creates a distinct pool directory on the library's filesystem volume.
// The pool owns its own mount request and files, independently of the source library resource.
func (suite *LibvirtSuite) provisionContentLibraryPool(ctx context.Context, node, library string) (string, string) {
	suite.T().Helper()
	suite.AssertServicesRunning(ctx, node, map[string]string{"ext-virtstoraged": "Running"})

	pool := storagecfg.NewStoragePoolConfigV1Alpha1()
	pool.MetaName = "copy-" + uuid.NewString()
	pool.VolumeConfig.VolumeName = library

	var target string

	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		nodeCtx := client.WithNode(cleanupCtx, node)
		suite.RemoveMachineConfigDocumentsByName(nodeCtx, storagecfg.StoragePoolConfigKind, pool.MetaName)
		rtestutils.AssertNoResource[*storage.StoragePoolStatus](nodeCtx, suite.T(), suite.Client.State(), pool.MetaName)

		if target != "" {
			output, code := suite.RunDebugContainer(cleanupCtx, node, "/nix/var/nix/profiles/default/bin/busybox", "rm", "-rf", "--", target)
			suite.Assert().Zero(code, "remove test-created pool directory: %s", output)
		}
	})
	suite.PatchMachineConfig(ctx, pool)
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), pool.MetaName,
		func(status *storage.StoragePoolStatus, asrt *assert.Assertions) {
			asrt.Equal(storage.StoragePoolPhaseReady, status.TypedSpec().Phase, status.TypedSpec().Error)

			if !asrt.Equal(pool.MetaName, filepath.Base(status.TypedSpec().TargetPath)) || !asrt.True(filepath.IsAbs(status.TypedSpec().TargetPath)) {
				return
			}

			target = filepath.Clean(status.TypedSpec().TargetPath)
		})

	return pool.MetaName, target
}

// diskStatusID is the ID the disk status of one configured disk is published under.
//
// The ID covers what the disk is provisioned from as well as its name, so it is built from the same
// document the test applied rather than written out by hand.
func diskStatusID(vm string, disk hypervisorcfg.VirtualMachineDisk) string {
	image := disk.ProvisionConfig.FromImageConfig

	return hypervisor.VirtualMachineDiskStatusID(vm, hypervisor.VirtualMachineDiskSpec{
		Name:   disk.Name(),
		Pool:   disk.Pool(),
		Format: disk.Format().String(),
		Type:   disk.Type().String(),
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{
				Library: image.ImageLibrary,
				File:    image.ImageFile,
				Digest:  image.ImageDigest,
				Mode:    image.Mode().String(),
			},
		},
	})
}
