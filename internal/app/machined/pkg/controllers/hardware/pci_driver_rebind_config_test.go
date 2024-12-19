// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hardwareconfigtype "github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	hardwareres "github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

type PCIDriverRebindConfigSuite struct {
	ctest.DefaultSuite
}

func TestPCIDriverRebindConfigSuite(t *testing.T) {
	suite.Run(t, new(PCIDriverRebindConfigSuite))
}

func (suite *PCIDriverRebindConfigSuite) TestPCIDriverRebindConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&hardware.PCIDriverRebindConfigController{}))

	pciDriverRebindConfig := &hardwareconfigtype.PCIDriverRebindConfigV1Alpha1{
		MetaName:        "0000:04:00.00",
		PCITargetDriver: "vfio-pci",
	}

	cfg, err := container.New(pciDriverRebindConfig)
	suite.Require().NoError(err)

	nCfg := config.NewMachineConfig(cfg)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), nCfg))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), pciDriverRebindConfig.MetaName, func(cfg *hardwareres.PCIDriverRebindConfig, asrt *assert.Assertions) {
		asrt.Equal(
			"0000:04:00.00",
			cfg.TypedSpec().PCIID,
		)
		asrt.Equal(
			"vfio-pci",
			cfg.TypedSpec().TargetDriver,
		)
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), nCfg.Metadata()))

	rtestutils.AssertNoResource[*hardwareres.PCIDriverRebindConfig](suite.Ctx(), suite.T(), suite.State(), pciDriverRebindConfig.MetaName)
}
