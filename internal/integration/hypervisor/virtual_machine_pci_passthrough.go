// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hardwarecfg "github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// passthroughTestProduct is the e1000 the QEMU provisioner attaches behind its emulated IOMMU
// when the cluster is created with IOMMU support.
const passthroughTestProduct = "82540EM Gigabit Ethernet Controller"

// passthroughTestDeviceID is the PCI device ID of that e1000, as the guest reads it from sysfs.
const passthroughTestDeviceID = "0x100e"

// TestAlpinePCIPassthrough binds the provisioner's IOMMU-attached e1000 to vfio-pci, passes it
// through to a stock Alpine guest on a Q35 machine, and checks the guest enumerates it.
func (suite *LibvirtSuite) TestAlpinePCIPassthrough() {
	node, nodeCtx := suite.requireAlpineGuestNode()

	if !suite.hostHasIOMMU(nodeCtx) {
		suite.T().Skip("skipping as IOMMU is not enabled on the node")
	}

	address := suite.findPassthroughTestDevice(nodeCtx)
	if address == "" {
		suite.T().Skipf("skipping as the node has no %q to pass through", passthroughTestProduct)
	}

	rebind := hardwarecfg.NewPCIDriverRebindConfigV1Alpha1()
	rebind.MetaName = address
	rebind.PCITargetDriver = hypervisorhelpers.VFIOPCIDriver

	// vfio_iommu_type1 is listed explicitly: it is only a soft dependency of vfio, which module
	// loading does not follow, and without it QEMU cannot set up the device's IOMMU group.
	vfioPCI := runtimecfg.NewKernelModuleConfigV1Alpha1("vfio_pci")
	vfioIOMMU := runtimecfg.NewKernelModuleConfigV1Alpha1("vfio_iommu_type1")

	// Registered before the guest is prepared, so it runs after the guest's own cleanup has
	// restored the configuration: the device is handed back to its host driver last.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cleanupCtx := client.WithNode(ctx, node)
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, hardwarecfg.PCIDriverRebindConfig, address)
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, runtimecfg.KernelModuleConfigKind, vfioPCI.Name(), vfioIOMMU.Name())
	})

	suite.PatchMachineConfig(nodeCtx, vfioPCI, vfioIOMMU, rebind)

	_, err := suite.Client.COSI.WatchFor(nodeCtx, hardware.NewPCIDriverRebindStatus(address).Metadata(),
		state.WithEventTypes(state.Created, state.Updated))
	suite.Require().NoError(err)

	// The default host profile selects q35, which presents the device behind a PCI Express root port.
	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20, "", func(doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
		doc.HostDevicesConfig.PCIConfig = []hypervisorcfg.VirtualMachinePCIDevice{{PCIAddress: address}}
	})
	name := doc.Name()

	// The guest's running state is asserted rather than required, so stop here when it never
	// started: logging in to its console would otherwise wait for a prompt that never comes.
	suite.Require().False(suite.T().Failed(), "virtual machine %q did not start", name)

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []resource.ID{name},
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(spec.TypedSpec().DomainXML, `machine="q35"`)
			asrt.Contains(spec.TypedSpec().DomainXML, `<hostdev mode="subsystem" type="pci" managed="no">`)
		},
	)

	console := suite.loginAlpineConsole(nodeCtx, name)

	defer func() {
		if suite.T().Failed() {
			suite.T().Logf("bounded Alpine console transcript: %q", console.transcript)
		}
	}()

	// The guest has no NIC of its own, so the only e1000 it can enumerate is the passed-through one.
	nonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	start := console.mark()
	suite.Require().NoError(console.send("grep -lx " + passthroughTestDeviceID + " /sys/bus/pci/devices/*/device >/dev/null && " +
		"printf '\\nALPINE_PCI_OK_%s\\n' '" + nonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nALPINE_PCI_OK_"+nonce+"\r\n")))
}

// hostHasIOMMU reports whether the node has at least one IOMMU registered.
func (suite *LibvirtSuite) hostHasIOMMU(nodeCtx context.Context) bool {
	suite.T().Helper()

	stream, err := suite.Client.LS(nodeCtx, &machineapi.ListRequest{Root: "/sys/class/iommu"})
	suite.Require().NoError(err)

	var count int

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, _ string, _ bool) error {
		if info.GetRelativeName() != "." {
			count++
		}

		return nil
	}))

	return count > 0
}

// findPassthroughTestDevice returns the PCI address of the provisioner's IOMMU-attached e1000,
// or an empty string when the node has none.
func (suite *LibvirtSuite) findPassthroughTestDevice(nodeCtx context.Context) string {
	suite.T().Helper()

	devices, err := suite.Client.COSI.List(nodeCtx,
		resource.NewMetadata(hardware.NamespaceName, hardware.PCIDeviceType, "", resource.VersionUndefined))
	suite.Require().NoError(err)

	for _, item := range devices.Items {
		device, ok := item.(*hardware.PCIDevice)
		suite.Require().True(ok, "expected PCI device, got %T", item)

		if device.TypedSpec().Product == passthroughTestProduct {
			return device.Metadata().ID()
		}
	}

	return ""
}
