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
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ImageSuite verifies writable disks provisioned from images.
type ImageSuite struct{ LibvirtSuite }

// SuiteName implements base.NamedSuite.
func (suite *ImageSuite) SuiteName() string { return "hypervisor.ImageSuite" }

// TestWritableImageFromContentLibrary runs import and guest I/O in short CI.
// Full CI also covers alias replacement, source-free node restart, offline
// growth and configuration withdrawal/re-addition with persistent guest writes.
func (suite *ImageSuite) TestWritableImageFromContentLibrary() {
	suite.requireContentLibrary()

	if suite.HypervisorDiskImagePath == "" {
		suite.T().Skip("-talos.hypervisor.disk-image is not set")
	}

	ctx, cancel := context.WithTimeout(suite.ctx, 30*time.Minute)
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP()

	nodeCtx := client.WithNode(ctx, node)
	if suite.ReadMachineArch(nodeCtx) != constants.ArchAmd64 {
		suite.T().Skip("stock OpenWrt fixture is amd64")
	}

	image, err := os.Open(suite.HypervisorDiskImagePath)
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		suite.Assert().NoError(image.Close())
	})

	// These helpers register library/backing and VM/domain/disk cleanup with T().Cleanup.
	// Short-mode returns and failed assertions still run that resource cleanup.
	library, libraryPath := provisionContentLibrary(&suite.APISuite, nodeCtx, node)
	fixture := newBlankDiskFixtureOnNode(ctx, &suite.LibvirtSuite, "vm-image", node)
	fixture.volumeFile = fixture.vmName + "__" + fixture.diskName + ".img"
	doc := fixture.writableImageDocument(library)

	upload := func(contents io.Reader) string {
		suite.T().Helper()
		response, err := suite.Client.ContentLibraryUpload(nodeCtx, library, "guest.img", "", contents)
		suite.Require().NoError(err)

		digest := response.GetDigests().GetSha256()
		suite.Require().Regexp(`^sha256:[a-f0-9]{64}$`, digest)

		return strings.TrimPrefix(digest, "sha256:")
	}
	deleteSource := func(digest string) {
		suite.T().Helper()
		_, err := suite.Client.ContentLibraryService().Delete(nodeCtx, &machine.ContentLibraryServiceDeleteRequest{
			LibraryId: library,
			Target:    &machine.ContentLibraryServiceDeleteRequest_Name{Name: "guest.img"},
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
		defer func() { suite.T().Logf("bounded OpenWrt transcript: %q", console.transcript) }()

		// sendUntil sends first, then resends resend every 10s until want arrives.
		// Input typed while boot scripts still run can be lost.
		sendUntil := func(first, resend string, want []byte) error {
			start := console.mark()
			if err := console.send(first); err != nil {
				return err
			}

			stopResend := make(chan struct{})
			resendDone := make(chan struct{})

			go func() {
				defer close(resendDone)

				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-stopResend:
						return
					case <-ticker.C:
					}

					if console.send(resend) != nil {
						return
					}
				}
			}()

			err := console.expectAfter(start, want)

			close(stopResend)
			<-resendDone

			return err
		}

		// OpenWrt gives a passwordless root shell on Enter. The "press Enter" banner is
		// printed once at boot and may predate the attach, so do not wait for it.
		suite.Require().NoError(sendUntil("\n", "\n", []byte(":~# ")))

		command := "grep -q OpenWrt /etc/openwrt_release && test \"$(cat /sys/class/block/vda/size)\" = " + size
		if write {
			command += " && echo " + fixture.vmName + " > /root/talos-sentinel && sync"
		} else {
			command += " && test \"$(cat /root/talos-sentinel)\" = " + fixture.vmName
		}

		marker := serialNonce("IMAGE")
		line := command + " && " + serialPrintf(marker)

		// ^C clears any partial line on resend; the command is idempotent.
		suite.Require().NoError(sendUntil(line, "\x03"+line, []byte("\r\n"+marker+"\r\n")))
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

	if testing.Short() || !suite.hostRebootSupported() {
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

func init() {
	allSuites = append(allSuites, new(ImageSuite))
}
