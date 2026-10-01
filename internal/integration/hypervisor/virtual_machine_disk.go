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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
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

// TestCDROMFromContentLibrary covers a cdrom sourced from a content library: the image is attached
// where it lies, so the domain names the library's own path and nothing is copied.
func (suite *LibvirtSuite) TestCDROMFromContentLibrary() {
	suite.requireContentLibrarySupport()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)

	contents := []byte("talos integration cdrom")
	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, false, "", bytes.NewReader(contents))
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

	source := filepath.Join(libraryPath, isoName)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, installDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
			asrt.Equal(source, status.TypedSpec().SourcePath)
			asrt.Equal("raw", status.TypedSpec().Format)
			asrt.True(status.TypedSpec().ReadOnly)
		},
	)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, name,
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

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.COSI,
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

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.False(status.TypedSpec().Ready)
			asrt.Contains(status.TypedSpec().Error, isoName)
		},
	)

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
	suite.assertNoDomain(node, name)

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, false, "", bytes.NewReader([]byte("late arrival")))
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

	if suite.HypervisorISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.iso is not set")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)

	info, err := os.Stat(suite.HypervisorISOPath)
	suite.Require().NoError(err)

	dgst := suite.digestOfISO()

	suite.T().Logf("uploading %s (%d bytes, %s) into content library %q", suite.HypervisorISOPath, info.Size(), dgst, library)

	iso, err := os.Open(suite.HypervisorISOPath)
	suite.Require().NoError(err)

	defer iso.Close() //nolint:errcheck

	started := time.Now()

	// The digest is declared, so the node verifies what it received before committing it.
	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, false, dgst.String(), iso)
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

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, doc.DisksConfig[0])},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
			asrt.Equal(filepath.Join(libraryPath, isoName), status.TypedSpec().SourcePath)
		},
	)

	// libvirt refuses to start a domain whose backing file it cannot open, so a running domain is
	// what proves QEMU reached the image through the library's mount and its labels.
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	// The guest holds the library file open; the image must still be listed as the library's own.
	suite.Require().Contains(
		suite.runVirsh(node, "domblklist", name, "--details"),
		filepath.Join(libraryPath, isoName),
	)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	suite.assertNoDomain(node, name)
}

// digestOfISO hashes the ISO in one streaming pass, so a large image never lands in memory.
func (suite *LibvirtSuite) digestOfISO() digest.Digest {
	suite.T().Helper()

	f, err := os.Open(suite.HypervisorISOPath)
	suite.Require().NoError(err)

	defer f.Close() //nolint:errcheck

	digester := digest.SHA256.Digester()

	_, err = io.Copy(digester.Hash(), f)
	suite.Require().NoError(err)

	return digester.Digest()
}

// TestCDROMImageIsHeldWhileAttached covers the holds which keep the medium under a running guest:
// the library file a domain is reading from cannot be deleted, and swapping the image frees the
// previous one only once the domain has actually been redefined.
func (suite *LibvirtSuite) TestCDROMImageIsHeldWhileAttached() {
	suite.requireContentLibrarySupport()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)

	const replacementISO = "replacement.iso"

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, false, "", bytes.NewReader([]byte("talos first cdrom")))
	suite.Require().NoError(err)

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, replacementISO, false, "", bytes.NewReader([]byte("talos second cdrom")))
	suite.Require().NoError(err)

	name := "vm-swap-" + uuid.NewString()

	installDisk := hypervisorcfg.VirtualMachineDisk{
		DiskName:      "install",
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

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, installDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
			asrt.Equal(filepath.Join(libraryPath, isoName), status.TypedSpec().SourcePath)
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	// The guest is reading from this file, so the content library refuses to take it away, by
	// deleting it or by overwriting it in place.
	_, err = suite.Client.ContentLibraryClient.Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Name:      isoName,
	})
	suite.Require().Error(err)
	suite.Require().Equal(codes.FailedPrecondition, grpcstatus.Code(err))

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, true, "", bytes.NewReader([]byte("talos overwritten")))
	suite.Require().Error(err)
	suite.Require().Equal(codes.FailedPrecondition, grpcstatus.Code(err))

	swappedDisk := installDisk
	swappedDisk.ProvisionConfig.FromImageConfig = &hypervisorcfg.VirtualMachineDiskFromImage{
		ImageLibrary: library,
		ImageFile:    replacementISO,
	}

	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{swappedDisk}
	suite.PatchMachineConfig(nodeCtx, doc)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, swappedDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
			asrt.Equal(filepath.Join(libraryPath, replacementISO), status.TypedSpec().SourcePath)
		},
	)

	// The status of the previous image is a resource of its own, and it goes once the domain has
	// been redefined without it.
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.COSI,
		diskStatusID(name, installDisk))

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)

	suite.Require().Contains(
		suite.runVirsh(node, "domblklist", name, "--details"),
		filepath.Join(libraryPath, replacementISO),
	)

	// Nothing reads the first image any more, so it may go.
	_, err = suite.Client.ContentLibraryClient.Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Name:      isoName,
	})
	suite.Require().NoError(err)

	// A stopped virtual machine reads nothing, so its image is the operator's to replace again. The
	// disk is still configured, and still has a status; what is gone is the hold on it.
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertNoDomain(node, name)

	_, err = suite.Client.ContentLibraryUpload(nodeCtx, library, replacementISO, true, "", bytes.NewReader([]byte("talos replaced")))
	suite.Require().NoError(err)

	_, err = suite.Client.ContentLibraryClient.Delete(nodeCtx, &machineapi.ContentLibraryServiceDeleteRequest{
		LibraryId: library,
		Name:      replacementISO,
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

	_, err := suite.Client.ContentLibraryUpload(nodeCtx, library, isoName, false, "", bytes.NewReader([]byte("talos held cdrom")))
	suite.Require().NoError(err)

	name := "vm-release-" + uuid.NewString()

	installDisk := hypervisorcfg.VirtualMachineDisk{
		DiskName:      "install",
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

	source := filepath.Join(libraryPath, isoName)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{diskStatusID(name, installDisk)},
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
			asrt.Equal(source, status.TypedSpec().SourcePath)
		},
	)

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 1, 0)
	suite.Require().Contains(suite.runVirsh(node, "domblklist", name, "--details"), source)

	// Every link of the chain is in place: the domain holds the disk status, the disk holds the
	// library status, and the library holds the mount its image is being read out of.
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{library},
		func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
			asrt.False(status.Metadata().Finalizers().Empty(), "the disk must hold the library")
		},
	)

	mountRequestID := contentLibraryControllerName + "/" + library + "/" + constants.UserVolumePrefix + library

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{mountRequestID},
		func(*block.VolumeMountStatus, *assert.Assertions) {},
	)

	// The guest goes, and every hold comes back with it. The library is only removable once no
	// virtual machine names it any more.
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	suite.assertNoDomain(node, name)

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDiskStatus](nodeCtx, suite.T(), suite.Client.COSI,
		diskStatusID(name, installDisk))

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.ContentLibraryConfigKind, library)

	rtestutils.AssertNoResource[*hypervisor.ContentLibraryStatus](nodeCtx, suite.T(), suite.Client.COSI, library)
	rtestutils.AssertNoResource[*block.VolumeMountRequest](nodeCtx, suite.T(), suite.Client.COSI, mountRequestID)
	rtestutils.AssertNoResource[*block.VolumeMountStatus](nodeCtx, suite.T(), suite.Client.COSI, mountRequestID)
}

// diskStatusID is the ID the disk status of one configured disk is published under.
//
// The ID covers what the disk is provisioned from as well as its name, so it is built from the same
// document the test applied rather than written out by hand.
func diskStatusID(vm string, disk hypervisorcfg.VirtualMachineDisk) string {
	image := disk.ProvisionConfig.FromImageConfig

	return hypervisor.VirtualMachineDiskStatusID(vm, hypervisor.VirtualMachineDiskSpec{
		Name: disk.Name(),
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
