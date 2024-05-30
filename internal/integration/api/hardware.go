// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
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

	for _, resourceType := range []resource.Type{
		hardware.MemoryModuleType,
		hardware.ProcessorType,
		hardware.PCIDeviceType,
	} {
		items, err := suite.Client.COSI.List(client.WithNode(suite.ctx, node), resource.NewMetadata(hardware.NamespaceName, resourceType, "", resource.VersionUndefined))
		suite.Require().NoError(err)

		suite.Assert().NotEmpty(items.Items, "resource type %s is not populated", resourceType)
	}
}

func init() {
	allSuites = append(allSuites, new(HardwareSuite))
}
