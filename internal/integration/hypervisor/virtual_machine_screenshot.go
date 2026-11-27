// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"bytes"
	"context"
	"image/png"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// ScreenshotSuite verifies capturing the display of a virtual machine.
type ScreenshotSuite struct{ LibvirtSuite }

// SuiteName implements base.NamedSuite.
func (suite *ScreenshotSuite) SuiteName() string { return "hypervisor.ScreenshotSuite" }

// SetupTest skips the suite in short mode: each test boots a guest.
func (suite *ScreenshotSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping screenshot tests in short mode")
	}

	suite.LibvirtSuite.SetupTest()
}

// prepareScreenshotVM starts a diskless guest, whose firmware keeps drawing to the display.
func (suite *ScreenshotSuite) prepareScreenshotVM(node string, vnc bool) (context.Context, string) {
	suite.T().Helper()

	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	name := "vm-screenshot-" + uuid.NewString()
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = suite.guestArch(node).firmware
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.ConsoleConfig.VNCConfig.VNCEnabled = new(vnc)

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertVirtualMachineStatus(nodeCtx, name, hypervisorhelpers.VirtualMachinePowerStateRunning)

	return nodeCtx, name
}

// TestScreenshotVideoEnabled captures a PNG from a guest with a video device.
func (suite *ScreenshotSuite) TestScreenshotVideoEnabled() {
	node := suite.RandomDiscoveredNodeInternalIP()

	// Talos renders a video device only on amd64; arm64 guests have no screen to capture yet.
	if suite.guestArch(node).qemu != constants.KernelArchAmd64 {
		suite.T().Skip("virtual machine video devices are only rendered on amd64")
	}

	nodeCtx, name := suite.prepareScreenshotVM(node, true)

	var resp *machine.VirtualMachineScreenshotResponse

	// The domain is reported running before QEMU has necessarily initialized its display.
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		var err error

		resp, err = suite.Client.HypervisorService().Screenshot(nodeCtx, &machine.VirtualMachineScreenshotRequest{Name: name})
		assert.NoError(collect, err)
	}, time.Minute, time.Second, "screenshot of %q must succeed", name)

	suite.Require().Equal("image/png", resp.GetMimeType())

	img, err := png.Decode(bytes.NewReader(resp.GetImage()))
	suite.Require().NoError(err, "screenshot must be a valid PNG")

	bounds := img.Bounds()
	suite.Require().Positive(bounds.Dx(), "screenshot must have a width")
	suite.Require().Positive(bounds.Dy(), "screenshot must have a height")
}

// TestScreenshotVideoDisabled requires a guest without VNC, and so without a video device, to be refused.
func (suite *ScreenshotSuite) TestScreenshotVideoDisabled() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx, name := suite.prepareScreenshotVM(node, false)

	_, err := suite.Client.HypervisorService().Screenshot(nodeCtx, &machine.VirtualMachineScreenshotRequest{Name: name})
	suite.Require().Equal(codes.FailedPrecondition, status.Code(err), "screenshot without video returned %v", err)
}

func init() {
	allSuites = append(allSuites, new(ScreenshotSuite))
}
