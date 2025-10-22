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
	hardwareconfigtype "github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// PCIDriverRebindSuite is a suite of tests for PCI rebind.
type PCIDriverRebindSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName returns the name of the suite.
func (suite *PCIDriverRebindSuite) SuiteName() string {
	return "api.PCIDriverRebindSuite"
}

// SetupTest sets up the test.
func (suite *PCIDriverRebindSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

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
func (suite *PCIDriverRebindSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestIOMMURebind tests PCI rebind.
func (suite *PCIDriverRebindSuite) TestIOMMURebind() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	nodeCtx := client.WithNode(suite.ctx, node)

	items, err := suite.Client.COSI.List(nodeCtx, resource.NewMetadata(hardware.NamespaceName, hardware.PCIDeviceType, "", resource.VersionUndefined))
	suite.Require().NoError(err)

	var pciDeviceID string

	for _, item := range items.Items {
		pci, ok := item.(*hardware.PCIDevice)
		suite.Require().True(ok, "expected PCI device, got %T", item)

		if pci.TypedSpec().Product == "82540EM Gigabit Ethernet Controller" {
			pciDeviceID = pci.Metadata().ID()

			break
		}
	}

	// validate that the driver is bound to e1000 initially
	suite.validateDriver(nodeCtx, pciDeviceID, "e1000")

	cfgDocument := hardwareconfigtype.NewPCIDriverRebindConfigV1Alpha1()
	cfgDocument.MetaName = pciDeviceID
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

	_, err = suite.Client.COSI.WatchFor(nodeCtx, hardware.NewPCIDriverRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Created, state.Updated))
	suite.Require().NoError(err)

	defer func() {
		suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

		suite.PatchMachineConfig(nodeCtx, map[string]any{
			"machine": map[string]any{
				"kernel": map[string]any{
					"$patch": "delete",
				},
			},
		})
	}()

	// after applying the patch the device should be bound to vfio-pci
	suite.validateDriver(nodeCtx, pciDeviceID, "vfio-pci")

	cfgDocument.PCITargetDriver = "e1000"

	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, hardware.NewPCIDriverRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Updated))
	suite.Require().NoError(err)

	// verify that an update to the target driver takes effect
	suite.validateDriver(nodeCtx, pciDeviceID, "e1000")

	// switch back to vfio-pci
	cfgDocument.PCITargetDriver = "vfio-pci"

	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, hardware.NewPCIDriverRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Updated))
	suite.Require().NoError(err)

	// verify that the update has taken effect
	suite.validateDriver(nodeCtx, pciDeviceID, "vfio-pci")

	// now remove the pci driver rebind config
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	_, err = suite.Client.COSI.WatchFor(nodeCtx, hardware.NewPCIDriverRebindStatus(pciDeviceID).Metadata(), state.WithEventTypes(state.Destroyed))
	suite.Require().NoError(err)

	// verify that the device is back to original host driver
	suite.validateDriver(nodeCtx, pciDeviceID, "e1000")
}

func (suite *PCIDriverRebindSuite) validateDriver(nodeCtx context.Context, pciDeviceID, driver string) {
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

	suite.Require().Equal(driver, filepath.Base(driverLink), "expected driver %q, got %q", driver, filepath.Base(driverLink))
}

func init() {
	allSuites = append(allSuites, &PCIDriverRebindSuite{})
}
