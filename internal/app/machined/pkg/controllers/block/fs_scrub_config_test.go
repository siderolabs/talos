// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type FSScrubConfigSuite struct {
	ctest.DefaultSuite
}

func TestFSScrubConfigSuite(t *testing.T) {
	suite.Run(t, new(FSScrubConfigSuite))
}

func (suite *FSScrubConfigSuite) TestFSScrubConfigNone() {
	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubConfigController{}))

	rtestutils.AssertNoResource[*block.FSScrubConfig](suite.Ctx(), suite.T(), suite.State(), "")
}

func (suite *FSScrubConfigSuite) TestFSScrubConfigMachineConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.FSScrubConfigController{}))

	FSScrubConfig := &blockcfg.FilesystemScrubV1Alpha1{
		MetaName:     "fsscrub",
		FSMountpoint: "/var",
	}

	cfg, err := container.New(FSScrubConfig)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResource[*block.FSScrubConfig](suite.Ctx(), suite.T(), suite.State(), "",
		func(cfg *block.FSScrubConfig, asrt *assert.Assertions) {
			asrt.Equal(
				"/var",
				cfg.TypedSpec().Mountpoint,
			)
			asrt.Equal(
				blockcfg.DefaultScrubPeriod,
				cfg.TypedSpec().Period,
			)
		})
}
