// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"time"

	"github.com/google/uuid"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// VideoSuite verifies the video device is attached independently of the VNC console.
type VideoSuite struct{ LibvirtSuite }

// SuiteName implements base.NamedSuite.
func (suite *VideoSuite) SuiteName() string { return "hypervisor.VideoSuite" }

// TestVideoDevice checks the live domain's video and graphics devices for each combination of
// `video.enabled` and `console.vnc`.
func (suite *VideoSuite) TestVideoDevice() {
	node := suite.RandomDiscoveredNodeInternalIP()

	// Talos renders the video device only on amd64.
	if suite.guestArch(node).qemu != constants.KernelArchAmd64 {
		suite.T().Skip("virtual machine video devices are only rendered on amd64")
	}

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	for _, tt := range []struct {
		name     string
		video    *bool
		vnc      bool
		videos   int
		graphics int
	}{
		{name: "video only", video: new(true), videos: 1},
		{name: "VNC implies video", vnc: true, videos: 1, graphics: 1},
		{name: "neither", videos: 0},
		{name: "video disabled", video: new(false), videos: 0},
	} {
		suite.Run(tt.name, func() {
			name := "vm-video-" + uuid.NewString()
			nodeCtx := client.WithNode(suite.ctx, node)

			doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
			doc.MetaName = name
			doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
			doc.FirmwareConfig.FirmwareType = suite.guestArch(node).firmware
			doc.CPUConfig.CPUCount = 1
			doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
			doc.ConsoleConfig.VNCConfig.VNCEnabled = new(tt.vnc)
			doc.VideoConfig.VideoEnabled = tt.video
			doc.VideoConfig.ModelConfig = "bochs"

			suite.T().Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
			})

			suite.PatchMachineConfig(nodeCtx, doc)
			suite.assertVirtualMachineStatus(nodeCtx, name, hypervisorhelpers.VirtualMachinePowerStateRunning)

			var domain libvirtxml.Domain

			suite.Require().NoError(domain.Unmarshal(suite.runVirsh(node, "dumpxml", name)))
			suite.Require().NotNil(domain.Devices)
			suite.Require().Len(domain.Devices.Graphics, tt.graphics)
			suite.Require().Len(domain.Devices.Videos, tt.videos)

			if tt.videos > 0 {
				suite.Require().Equal("bochs", domain.Devices.Videos[0].Model.Type, "the configured model must be rendered")
			}
		})
	}
}

func init() {
	allSuites = append(allSuites, new(VideoSuite))
}
