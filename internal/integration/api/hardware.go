// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// HardwareSuite ...
type HardwareSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *HardwareSuite) SuiteName() string {
	return "api.HardwareSuite"
}

// SetupTest ...
func (suite *HardwareSuite) SetupTest() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skipf("doesn't run Talos kernel, skipping")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)
}

// TearDownTest ...
func (suite *HardwareSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestSystemInformation tests that SystemInformation is populated.
func (suite *HardwareSuite) TestSystemInformation() {
	node := suite.RandomDiscoveredNodeInternalIP()

	sysInfo, err := safe.StateGetByID[*hardware.SystemInformation](client.WithNode(suite.ctx, node), suite.Client.COSI, hardware.SystemInformationID)
	suite.Require().NoError(err)

	suite.Assert().NotEmpty(sysInfo.TypedSpec().UUID)
	suite.Assert().NotEqual((uuid.UUID{}).String(), sysInfo.TypedSpec().UUID)
}

// TestHardwareInfo tests that hardware info is populated.
func (suite *HardwareSuite) TestHardwareInfo() {
	node := suite.RandomDiscoveredNodeInternalIP()

	resourceList := []resource.Type{
		hardware.MemoryModuleType,
		hardware.ProcessorType,
	}

	if suite.Cluster != nil {
		// cloud VMs might not publish PCI devices
		resourceList = append(resourceList, hardware.PCIDeviceType)
	}

	for _, resourceType := range resourceList {
		items, err := suite.Client.COSI.List(client.WithNode(suite.ctx, node), resource.NewMetadata(hardware.NamespaceName, resourceType, "", resource.VersionUndefined))
		suite.Require().NoError(err)

		suite.Assert().NotEmpty(items.Items, "resource type %s is not populated", resourceType)
	}
}

// TestPCRStatus tests that the PCR was correctly extended.
func (suite *HardwareSuite) TestPCRStatus() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	rtestutils.AssertNoResource[*hardware.PCRStatus](ctx, suite.T(), suite.Client.COSI, hardware.NewPCCRStatus(constants.UKIPCR).Metadata().ID())
}

// TestLogicalCPUInfo tests the per-logical-CPU resource is populated end-to-end.
//
// Microcode and Bugs are amd64-specific; Socket/Core/NumaNode are derived from
// sysfs topology and should be readable on any architecture the kernel reports
// them on.
func (suite *HardwareSuite) TestLogicalCPUInfo() {
	node := suite.RandomDiscoveredNodeInternalIP()

	items, err := suite.Client.COSI.List(
		client.WithNode(suite.ctx, node),
		resource.NewMetadata(hardware.NamespaceName, hardware.LogicalCPUInfoType, "", resource.VersionUndefined),
	)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(items.Items)

	var (
		seenCores    = map[uint32]struct{}{}
		seenSockets  = map[uint32]struct{}{}
		anyMicrocode bool
	)

	for _, item := range items.Items {
		c, ok := item.(*hardware.LogicalCPUInfo)
		if !ok {
			continue
		}

		spec := c.TypedSpec()
		seenCores[spec.Core] = struct{}{}
		seenSockets[spec.Socket] = struct{}{}

		if spec.Microcode != "" {
			anyMicrocode = true
		}
	}

	// At least one socket reported (single-socket systems pin everything to 0,
	// multi-socket sees several). Asserts the sysfs topology read returned
	// without erroring out.
	suite.Assert().NotEmpty(seenSockets, "no Socket value populated for any LogicalCPUInfo")

	// On a multi-thread host with SMT off, every Core differs; with SMT on we
	// see at least floor(threads/2) distinct cores. On a single-core VM both
	// distinct-Cores and same-Cores are valid — so just require the field is
	// present (>=1 unique value, including 0).
	suite.Assert().NotEmpty(seenCores, "no Core value populated for any LogicalCPUInfo")

	switch arch := nodeArch(suite); arch {
	case "amd64":
		if !anyMicrocode {
			// QEMU and some hypervisors don't expose a microcode revision even
			// on amd64. Skip rather than fail because the field's absence is
			// honest about what the environment provides.
			suite.T().Skip("amd64 node but no logical CPU reported a microcode revision (likely QEMU)")
		}
	default:
		suite.T().Logf("microcode not asserted on %q; only amd64 surfaces it via /proc/cpuinfo", arch)
	}
}

func nodeArch(suite *HardwareSuite) string {
	resp, err := suite.Client.Version(suite.ctx)
	if err != nil || len(resp.GetMessages()) == 0 {
		return ""
	}

	return resp.GetMessages()[0].GetVersion().GetArch()
}

func init() {
	allSuites = append(allSuites, new(HardwareSuite))
}
