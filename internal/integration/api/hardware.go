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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
		hardware.CPUCoreType,
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

// TestBIOSVersion tests that SystemInformation.BIOSVersion is populated.
//
// BIOSVersion comes from SMBIOS. SMBIOS is essentially always present on
// amd64, so we require non-empty there. On other architectures (most embedded
// arm64 hardware exposes no SMBIOS) we accept empty.
func (suite *HardwareSuite) TestBIOSVersion() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	sysInfo, err := safe.StateGetByID[*hardware.SystemInformation](
		ctx, suite.Client.COSI, hardware.SystemInformationID,
	)
	suite.Require().NoError(err)

	if arch := suite.ReadMachineArch(ctx); arch != "amd64" {
		suite.T().Skipf("skipping BIOS version check on arch %q", arch)
	}

	suite.Assert().NotEmpty(sysInfo.TypedSpec().BIOSVersion, "amd64 node reported no BIOSVersion")
}

// TestDiskFirmwareVersion tests that Disk.FirmwareVersion is populated for NVMe disks.
func (suite *HardwareSuite) TestDiskFirmwareVersion() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	items, err := safe.StateListAll[*block.Disk](
		ctx,
		suite.Client.COSI,
	)
	suite.Require().NoError(err)

	var nvmeChecked int

	for disk := range items.All() {
		if disk.TypedSpec().Transport != "nvme" {
			continue
		}

		nvmeChecked++

		suite.Assert().NotEmpty(disk.TypedSpec().FirmwareVersion)
	}

	if nvmeChecked == 0 {
		suite.T().Skip("no NVMe disks discovered")
	}
}

// TestDiskSMARTStatus tests that SMART status is populated for disks that support it, and
// that a disk found in standby is reported as such without being spun up just to be probed
// (see SMARTStatusSpec.PowerState/Message).
//
// SMART status resources are produced if and only if a DiskSMARTConfig document is present in
// the machine config (Talos >= 1.15 emits it by default, so SMART is on unless the document was
// removed). This test reads the config state either way instead of assuming SMART is always on.
func (suite *HardwareSuite) TestDiskSMARTStatus() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	cfg, err := safe.StateGetByID[*config.MachineConfig](ctx, suite.Client.COSI, config.ActiveID)
	suite.Require().NoError(err)

	// the presence of the DiskSMARTConfig document is what enables SMART collection.
	smartEnabled := cfg.Config().DiskSMARTConfig() != nil

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	var checked int

	for disk := range disks.All() {
		if disk.TypedSpec().CDROM || disk.TypedSpec().DevPath == "" {
			continue
		}

		if suite.assertDiskSMARTStatus(ctx, disk, smartEnabled) {
			checked++
		}
	}

	if smartEnabled && checked == 0 {
		suite.T().Skip("no disks with SMART support discovered")
	}
}

// assertDiskSMARTStatus asserts the SMART status of a single disk against the expected
// SMART-enabled state, and reports whether the disk was actually checked (i.e. it supports
// SMART and SMART is enabled).
func (suite *HardwareSuite) assertDiskSMARTStatus(ctx context.Context, disk *block.Disk, smartEnabled bool) bool {
	status, err := safe.StateGetByID[*block.SMARTStatus](ctx, suite.Client.COSI, disk.Metadata().ID())

	if !smartEnabled {
		// SMART is disabled: no SMARTStatus resource must ever be produced, for any disk.
		suite.Assert().True(state.IsNotFoundError(err), "disk %q: unexpected SMART status while SMART is disabled", disk.Metadata().ID())

		return false
	}

	if err != nil {
		if state.IsNotFoundError(err) {
			// some disks (e.g. virtio) don't support SMART, this is expected.
			return false
		}

		suite.Require().NoError(err)
	}

	spec := status.TypedSpec()

	suite.Assert().Equal(disk.TypedSpec().DevPath, spec.DevPath, "disk %q", disk.Metadata().ID())
	suite.Assert().NotEmpty(spec.DevType, "disk %q", disk.Metadata().ID())

	if spec.PowerState == "standby" {
		// a standby disk must never be spun up just to be probed: no fresh SMART
		// data is read, only the power state/message are refreshed.
		suite.Assert().Equal("skipped: disk in standby", spec.Message, "disk %q", disk.Metadata().ID())
	}

	return true
}

// TestDiskSMARTStatusDisabled verifies that SMART status collection is driven by the presence of
// the DiskSMARTConfig document: removing the document reaps every SMARTStatus resource on the
// node, and restoring the document brings them back.
func (suite *HardwareSuite) TestDiskSMARTStatusDisabled() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	// this test applies the machine config twice and waits for the controller to reconcile in
	// between, which doesn't fit the short read-only budget SetupTest allocates.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	suite.ClearConnectionRefused(ctx, node)

	nodeCtx := client.WithNode(ctx, node)

	cfg, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if cfg.DiskSMARTConfig() == nil {
		suite.T().Skip("DiskSMARTConfig document is not present, SMART is already disabled")
	}

	statuses, err := safe.StateListAll[*block.SMARTStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	if statuses.Len() == 0 {
		suite.T().Skip("no disks with SMART support discovered")
	}

	// keep the document as it is on the node, so it can be re-applied verbatim.
	var originalDocument any

	for _, doc := range cfg.Documents() {
		if doc.Kind() == blockcfg.DiskSMARTConfigKind {
			originalDocument = doc

			break
		}
	}

	suite.Require().NotNil(originalDocument, "DiskSMARTConfig document not found in the machine config")

	// guarantee the document is restored even if an assertion below fails partway through;
	// re-applying the same document is idempotent, so the explicit restore on the happy path
	// is harmless.
	defer func() {
		restoreCtx, restoreCancel := context.WithTimeout(context.Background(), time.Minute)
		defer restoreCancel()

		suite.T().Logf("ensuring DiskSMARTConfig is restored on node %s", node)
		suite.PatchMachineConfig(client.WithNode(restoreCtx, node), originalDocument)
	}()

	suite.T().Logf("removing DiskSMARTConfig from node %s", node)
	suite.RemoveMachineConfigDocuments(nodeCtx, blockcfg.DiskSMARTConfigKind)

	// without the document SMART is disabled: no status is collected, and the statuses collected
	// earlier are reaped.
	rtestutils.AssertLength[*block.SMARTStatus](nodeCtx, suite.T(), suite.Client.COSI, 0)

	suite.T().Logf("restoring DiskSMARTConfig on node %s", node)
	suite.PatchMachineConfig(nodeCtx, originalDocument)

	// with the document back, the same set of statuses is collected again.
	rtestutils.AssertLength[*block.SMARTStatus](nodeCtx, suite.T(), suite.Client.COSI, statuses.Len())
}

// TestBMCDevice tests that a BMC is discovered over the local IPMI interface.
//
// It requires a cluster created with `--with-ipmi`, which attaches QEMU's built-in BMC
// simulator with the identity asserted below.
func (suite *HardwareSuite) TestBMCDevice() {
	if !suite.IPMI {
		suite.T().Skip("cluster is running without IPMI emulation, skipping")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	bmc, err := safe.StateGetByID[*hardware.BMCDevice](ctx, suite.Client.COSI, "ipmi0")
	suite.Require().NoError(err)

	suite.Assert().EqualValues(674, bmc.TypedSpec().ManufacturerID)
	suite.Assert().Equal("Dell", bmc.TypedSpec().Manufacturer)
	suite.Assert().EqualValues(666, bmc.TypedSpec().ProductID)
	suite.Assert().Equal("7.10", bmc.TypedSpec().FirmwareVersion)
	suite.Assert().Equal("2.0", bmc.TypedSpec().IPMIVersion)

	// the QEMU BMC simulator doesn't implement Get LAN Configuration Parameters,
	// so the network configuration is expected to be empty
	suite.Assert().False(bmc.TypedSpec().Address.IsValid())
	suite.Assert().False(bmc.TypedSpec().Gateway.IsValid())
}

func init() {
	allSuites = append(allSuites, new(HardwareSuite))
}
