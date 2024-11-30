// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/pci"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type PCIRebindConfigSuite struct {
	ctest.DefaultSuite
}

func TestPCIRebindConfigSuite(t *testing.T) {
	suite.Run(t, new(PCIRebindConfigSuite))
}

func (suite *PCIRebindConfigSuite) TestPCIRebindConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.PCIRebindConfigController{}))

	pciRebindConfig := &pci.RebindConfigV1Alpha1{
		MetaName:          "ixgbe-bind",
		PCIVendorDeviceID: "0000:04:00.00",
		PCITargetDriver:   "vfio-pci",
	}

	cfg, err := container.New(pciRebindConfig)
	suite.Require().NoError(err)

	nCfg := config.NewMachineConfig(cfg)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nCfg))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), pciRebindConfig.MetaName, func(cfg *runtime.PCIRebindConfig, asrt *assert.Assertions) {
		asrt.Equal(
			"ixgbe-bind",
			cfg.TypedSpec().Name,
		)
		asrt.Equal(
			"0000:04:00.00",
			cfg.TypedSpec().VendorDeviceID,
		)
		asrt.Equal(
			"vfio-pci",
			cfg.TypedSpec().TargetDriver,
		)
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), nCfg.Metadata()))

	rtestutils.AssertNoResource[*runtime.PCIRebindConfig](suite.Ctx(), suite.T(), suite.State(), pciRebindConfig.MetaName)
}
