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

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// isoName is the library file the cdrom tests attach. Its contents are not a real ISO: nothing
// boots it, and the domain only has to name it.
const isoName = "install.iso"

// requireContentLibrarySupport skips unless the cluster can hold the library these tests source
// their image from. LibvirtSuite itself only requires the hypervisor.
func (suite *LibvirtSuite) requireContentLibrarySupport() {
	suite.T().Helper()

	if testing.Short() {
		suite.T().Skip("skipping test in short mode.")
	}

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
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
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
		},
	}

	// Register cleanup before applying configuration: a failed apply can still leave a guest
	// behind. Do not reuse the test's expiring context.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)

	source := filepath.Join(libraryPath, isoName)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{hypervisor.VirtualMachineDiskStatusID(name, "install")},
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
		hypervisor.VirtualMachineDiskStatusID(name, "install"))
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

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{hypervisor.VirtualMachineDiskStatusID(name, "install")},
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

	if suite.HypervisorTalosISOPath == "" {
		suite.T().Skip("skipping as -talos.hypervisor.talos-iso is not set")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)

	info, err := os.Stat(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	dgst := suite.digestOfISO()

	suite.T().Logf("uploading %s (%d bytes, %s) into content library %q", suite.HypervisorTalosISOPath, info.Size(), dgst, library)

	iso, err := os.Open(suite.HypervisorTalosISOPath)
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

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{hypervisor.VirtualMachineDiskStatusID(name, "install")},
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

	f, err := os.Open(suite.HypervisorTalosISOPath)
	suite.Require().NoError(err)

	defer f.Close() //nolint:errcheck

	digester := digest.SHA256.Digester()

	_, err = io.Copy(digester.Hash(), f)
	suite.Require().NoError(err)

	return digester.Digest()
}
