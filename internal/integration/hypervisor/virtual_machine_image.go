// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func (f *blankDiskFixture) writableImageDocument(library string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := f.document("2GiB", hypervisorhelpers.PowerStateRunning)
	doc.MemoryConfig.MemorySize = meta.MustByteSize("256MiB")
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.DisksConfig[0].ProvisionConfig = hypervisorcfg.VirtualMachineDiskProvision{
		FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
			ImageLibrary: library,
			ImageFile:    "guest.qcow2",
		},
	}

	return doc
}

// TestWritableImageFromContentLibrary runs import and guest I/O in short CI.
// Full CI also covers alias replacement, source-free node restart, offline
// growth and configuration withdrawal/re-addition with persistent guest writes.
func (suite *LibvirtSuite) TestWritableImageFromContentLibrary() {
	suite.requireContentLibrary()

	if suite.HypervisorDiskImagePath == "" {
		suite.T().Skip("-talos.hypervisor.disk-image is not set")
	}

	ctx, cancel := context.WithTimeout(suite.ctx, 30*time.Minute)
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP()

	nodeCtx := client.WithNode(ctx, node)
	if suite.ReadMachineArch(nodeCtx) != "amd64" {
		suite.T().Skip("stock CirrOS fixture is amd64")
	}

	image, err := os.Open(suite.HypervisorDiskImagePath)
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		suite.Assert().NoError(image.Close())
	})

	// These helpers register library/backing and VM/domain/disk cleanup with T().Cleanup.
	// Short-mode returns and failed assertions still run that resource cleanup.
	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	fixture := newBlankDiskFixtureOnNode(ctx, suite, "vm-image", node)
	fixture.volumeFile = fixture.vmName + "__" + fixture.diskName + ".img"
	doc := fixture.writableImageDocument(library)

	upload := func(contents io.Reader) string {
		suite.T().Helper()
		response, err := suite.Client.ContentLibraryUpload(nodeCtx, library, "guest.qcow2", "", contents)
		suite.Require().NoError(err)

		digest := response.GetDigests().GetSha256()
		suite.Require().Regexp(`^sha256:[a-f0-9]{64}$`, digest)

		return strings.TrimPrefix(digest, "sha256:")
	}
	deleteSource := func(digest string) {
		suite.T().Helper()
		_, err := suite.Client.ContentLibraryService().Delete(nodeCtx, &machine.ContentLibraryServiceDeleteRequest{
			LibraryId: library,
			Target:    &machine.ContentLibraryServiceDeleteRequest_Name{Name: "guest.qcow2"},
		})
		suite.Require().NoError(err)
		_, err = suite.Client.ContentLibraryService().Delete(nodeCtx, &machine.ContentLibraryServiceDeleteRequest{
			LibraryId: library,
			Target:    &machine.ContentLibraryServiceDeleteRequest_Sha256{Sha256: digest},
		})
		suite.Require().NoError(err)
	}
	assertDisk := func(size uint64) {
		suite.T().Helper()
		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), diskStatusID(fixture.vmName, doc.DisksConfig[0]),
			func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
				asrt.Equal(hypervisor.VirtualMachineDiskPhaseReady, status.TypedSpec().Phase, status.TypedSpec().Error)
				asrt.Equal(filepath.Join(fixture.poolPath, fixture.volumeFile), status.TypedSpec().SourcePath)
				asrt.Equal("qcow2", status.TypedSpec().Format)
				asrt.False(status.TypedSpec().ReadOnly)
				asrt.Equal(size, status.TypedSpec().Size)
			})
	}
	checkGuest := func(size string, write bool) {
		suite.T().Helper()

		suite.assertRunningTransientDomainWithDevices(node, fixture.vmName, 1, 1, 0)

		console := suite.attachAlpineConsole(nodeCtx, fixture.vmName)
		defer console.cancel()
		defer func() { suite.T().Logf("bounded CirrOS transcript: %q", console.transcript) }()

		suite.Require().NoError(console.send("\n"))
		suite.Require().NoError(console.expectAfter(0, []byte("login:")))
		start := console.mark()
		suite.Require().NoError(console.send("cirros\n"))
		suite.Require().NoError(console.expectAfter(start, []byte("Password:")))
		start = console.mark()
		suite.Require().NoError(console.send("gocubsgo\n"))
		suite.Require().NoError(console.expectAfter(start, []byte("$ ")))

		command := "grep -q '^cirros:' /etc/passwd && test \"$(cat /sys/class/block/vda/size)\" = " + size
		if write {
			command += " && echo " + fixture.vmName + " > /home/cirros/talos-sentinel && sync"
		} else {
			command += " && test \"$(cat /home/cirros/talos-sentinel)\" = " + fixture.vmName
		}

		marker := serialNonce("IMAGE")
		start = console.mark()
		suite.Require().NoError(console.send(command + " && " + serialPrintf(marker)))
		suite.Require().NoError(console.expectAfter(start, []byte("\r\n"+marker+"\r\n")))
		suite.Require().NoError(console.send("exit\n"))
		suite.detachAlpineConsole(console)
	}

	imageDigest := upload(image)

	suite.PatchMachineConfig(nodeCtx, doc)
	assertDisk(2 << 30)
	suite.Require().EqualValues(2<<30, fixture.virtualSize(nodeCtx))
	volumeXML := suite.runVirsh(node, "vol-dumpxml", fixture.volumeFile, "--pool", fixture.poolName)
	suite.Require().Contains(volumeXML, `type='qcow2'`)
	suite.Require().NotContains(volumeXML, "<backingStore>")
	suite.assertRunningTransientDomainWithDevices(node, fixture.vmName, 1, 1, 0)
	domainXML := suite.runVirsh(node, "dumpxml", fixture.vmName)
	suite.Require().Contains(domainXML, `device='disk'`)
	suite.Require().Contains(domainXML, filepath.Join(fixture.poolPath, fixture.volumeFile))
	checkGuest("4194304", true)

	output, code := suite.RunDebugContainer(nodeCtx, node, "/nix/var/nix/profiles/default/bin/busybox", "sha256sum",
		filepath.Join(libraryPath, "objects", "sha256", imageDigest))
	suite.Require().Zero(code, "%s", output)
	suite.Require().Contains(output, imageDigest)

	if testing.Short() {
		return
	}

	deleteSource("sha256:" + imageDigest)

	replacement := []byte("replacement is deliberately not a boot disk")
	upload(bytes.NewReader(replacement))
	assertDisk(2 << 30)
	suite.AssertRebootedNoChecks(ctx, node, func(rebootCtx context.Context) error {
		return base.IgnoreGRPCUnavailable(suite.Client.Reboot(rebootCtx))
	}, 5*time.Minute)
	suite.WaitForBootDone(ctx)
	assertDisk(2 << 30)
	checkGuest("4194304", false)

	deleteSource(fmt.Sprintf("sha256:%x", sha256.Sum256(replacement)))
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.ContentLibraryConfigKind, library)

	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertNoDomainWithContext(ctx, node, fixture.vmName)

	doc.DisksConfig[0].DiskSize = meta.MustByteSize("3GiB")
	suite.PatchMachineConfig(nodeCtx, doc)
	assertDisk(3 << 30)
	suite.Require().EqualValues(3<<30, fixture.virtualSize(nodeCtx))
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, fixture.vmName)
	removeBlankDiskVolume(nodeCtx, suite.T(), suite.Client.State(), fixture.volumeStatusID(), func() {
		suite.Require().True(fixture.fileExists(nodeCtx))
	})

	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	suite.PatchMachineConfig(nodeCtx, doc)
	assertDisk(3 << 30)
	checkGuest("6291456", false)
}
