// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// TestAlpineConsoleBlankDiskFormat formats only the disposable blank /dev/vda
// from the stock Alpine live ISO, then verifies a write across unmount/remount.
func (suite *LibvirtSuite) TestAlpineConsoleBlankDiskFormat() {
	node, nodeCtx := suite.requireAlpineGuestNode()
	// Reuse the ISO upload and cleanup, without a DHCP reservation or guest networking.
	live := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20)
	live.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, live)
	suite.assertStoppedDomain(nodeCtx, node, live.Name())
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, live.Name())

	disk := newBlankDiskFixtureOnNode(suite.ctx, suite, "vm-alpine-format", node)
	doc := disk.document("128MiB", hypervisorhelpers.PowerStateRunning)
	doc.MemoryConfig.MemorySize = meta.MustByteSize("512MiB")
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.DisksConfig[0].DiskBootOrder = 2
	isoDisk := live.DisksConfig[0]
	isoDisk.DiskBootOrder = 1
	doc.DisksConfig = append(doc.DisksConfig, isoDisk)

	suite.PatchMachineConfig(nodeCtx, doc)
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, disk.diskStatusID(),
		func(status *hypervisor.VirtualMachineDiskStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Phase == hypervisor.VirtualMachineDiskPhaseReady, "blank disk: %s", status.TypedSpec().Error)
			asrt.True(status.TypedSpec().Blank)
		})
	suite.assertRunningTransientDomainWithDevices(node, disk.vmName, 1, 2, 0)

	streamCtx, cancel := context.WithTimeout(nodeCtx, 3*time.Minute)
	defer cancel()

	stream, err := suite.Client.HypervisorClient.ConsoleStream(streamCtx)
	suite.Require().NoError(err)
	suite.Require().NoError(stream.Send(&machine.ConsoleRequest{Request: &machine.ConsoleRequest_Attach{
		Attach: &machine.ConsoleAttach{Name: disk.vmName},
	}}))
	console := &alpineConsole{stream: stream, cancel: cancel}

	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("format serial transcript (bounded): %q", console.transcript)
			suite.T().Logf("format VM log: %q", suite.vmLogSnapshot(nodeCtx, disk.vmName, 200))
		}

		cancel()
	}()

	suite.Require().NoError(console.send("\n"))
	suite.Require().NoError(console.expectAfter(0, []byte("login:")))
	start := console.mark()
	suite.Require().NoError(console.send("root\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("localhost:~#")))

	nonce := make([]byte, 16)
	_, err = rand.Read(nonce)
	suite.Require().NoError(err)

	marker := hex.EncodeToString(nonce)

	// The pinned ISO's alpine-base installs BusyBox, whose mkfs.vfat applet is
	// present on the ISO. e2fsprogs is on the media but not installed by default.
	// Keep every command and any failure diagnostics on serial, before the exit
	// marker. The marker's expanded status cannot appear in echoed shell input.
	start = console.mark()
	suite.Require().NoError(console.send("mkfs.vfat /dev/vda && mkdir -p /mnt/talos-disk && " +
		"mount -t vfat /dev/vda /mnt/talos-disk && " +
		"printf '%s' '" + marker + "' >/mnt/talos-disk/talos-marker && " +
		"sync && umount /mnt/talos-disk && " +
		"mount -t vfat /dev/vda /mnt/talos-disk && " +
		"test \"$(cat /mnt/talos-disk/talos-marker)\" = '" + marker + "'; " +
		"rc=$?; if [ \"$rc\" -ne 0 ]; then mount; dmesg | tail -n 30; fi; " +
		"printf '\\nFORMAT_EXIT_%s_%s_END\\n' '" + marker + "' \"$rc\"\n"))
	prefix := "\r\nFORMAT_EXIT_" + marker + "_"
	suite.Require().NoError(console.expectAfter(start, []byte(prefix)), "format/readback did not complete within the console deadline")
	completion := start + strings.Index(string(console.transcript[start:]), prefix) + len(prefix)
	suite.Require().NoError(console.expectAfter(completion, []byte("_END\r\n")))
	suite.Require().Equal("0", strings.SplitN(string(console.transcript[completion:]), "_END\r\n", 2)[0], "format, remount or readback failed")
	suite.detachAlpineConsole(console)
}
