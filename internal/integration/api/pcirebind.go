// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/pci"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// PCIRebindSuite is a suite of tests for PCI rebind.
type PCIRebindSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName returns the name of the suite.
func (suite *PCIRebindSuite) SuiteName() string {
	return "api.PCIRebindSuite"
}

// SetupTest sets up the test.
func (suite *PCIRebindSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping virtio test since provisioner is not qemu")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	nodeCtx := client.WithNode(suite.ctx, node)

	stream, err := suite.Client.LS(nodeCtx, &machineapi.ListRequest{
		Root: "/sys/class/iommu",
	})

	suite.Require().NoError(err)

	var count int

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
		if info.GetRelativeName() == "." {
			return nil
		}

		count++

		return nil
	}))

	if count == 0 {
		suite.T().Skip("skipping PCI rebind test since IOMMU is not enabled")
	}
}

// TearDownTest tears down the test.
func (suite *PCIRebindSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestIOMMURebind tests PCI rebind.
func (suite *PCIRebindSuite) TestIOMMURebind() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	nodeCtx := client.WithNode(suite.ctx, node)

	items, err := suite.Client.COSI.List(nodeCtx, resource.NewMetadata(hardware.NamespaceName, hardware.PCIDeviceType, "", resource.VersionUndefined))
	suite.Require().NoError(err)

	var pciDeviceID string

	for _, item := range items.Items {
		pci, ok := item.(*hardware.PCIDevice)
		suite.Require().True(ok, "expected PCI device, got %T", item)

		if pci.TypedSpec().Product == "Virtio 1.0 network device" {
			pciDeviceID = pci.Metadata().ID()

			break
		}
	}

	suite.validateDriver(nodeCtx, pciDeviceID, "virtio-pci")

	cfgDocument := pci.NewRebindConfigV1Alpha1()
	cfgDocument.MetaName = pciDeviceID
	cfgDocument.PCIVendorDeviceID = pciDeviceID
	cfgDocument.PCITargetDriver = "vfio-pci"

	v1alpha1CfgDocument := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKernel: &v1alpha1.KernelConfig{
				KernelModules: []*v1alpha1.KernelModuleConfig{
					{
						ModuleName: "vfio-pci",
					},
				},
			},
		},
	}

	suite.PatchMachineConfig(nodeCtx, v1alpha1CfgDocument, cfgDocument)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewPCIRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Created, state.Updated))
	suite.Require().NoError(err)

	suite.validateDriver(nodeCtx, pciDeviceID, "vfio-pci")

	cfgDocument.PCITargetDriver = "virtio-pci"

	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewPCIRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Updated))
	suite.Require().NoError(err)

	suite.validateDriver(nodeCtx, pciDeviceID, "virtio-pci")

	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, runtimeres.NewPCIRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Destroyed))
	suite.Require().NoError(err)

	suite.validateDriver(nodeCtx, pciDeviceID, "virtio-pci")
}

func (suite *PCIRebindSuite) validateDriver(nodeCtx context.Context, pciDeviceID, driver string) {
	stream, err := suite.Client.LS(nodeCtx, &machineapi.ListRequest{
		Root:  fmt.Sprintf("/sys/bus/pci/devices/%s", pciDeviceID),
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_SYMLINK},
	})
	suite.Require().NoError(err)

	var driverLink string

	suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
		if info.GetRelativeName() == "driver" {
			driverLink = info.Link

			return nil
		}

		return nil
	}))

	suite.Require().Equal(driver, filepath.Base(driverLink), "expected driver %q, got %q", driver, driverLink)
}

func init() {
	allSuites = append(allSuites, &PCIRebindSuite{})
}
