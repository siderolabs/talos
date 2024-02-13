// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type WatchdogTimerConfigSuite struct {
	ctest.DefaultSuite
}

func TestWatchdogTimerConfigSuite(t *testing.T) {
	suite.Run(t, new(WatchdogTimerConfigSuite))
}

func (suite *WatchdogTimerConfigSuite) TestWatchdogTimerConfigNone() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.WatchdogTimerConfigController{}))

	rtestutils.AssertNoResource[*runtime.WatchdogTimerConfig](suite.Ctx(), suite.T(), suite.State(), runtime.WatchdogTimerConfigID)
}

func (suite *WatchdogTimerConfigSuite) TestWatchdogTimerConfigMachineConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.WatchdogTimerConfigController{}))

	watchdogTimerConfig := &runtimecfg.WatchdogTimerV1Alpha1{
		WatchdogDevice: "/dev/watchdog0",
	}

	cfg, err := container.New(watchdogTimerConfig)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResources[*runtime.WatchdogTimerConfig](suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.WatchdogTimerConfigID},
		func(cfg *runtime.WatchdogTimerConfig, asrt *assert.Assertions) {
			asrt.Equal(
				"/dev/watchdog0",
				cfg.TypedSpec().Device,
			)
			asrt.Equal(
				runtimecfg.DefaultWatchdogTimeout,
				cfg.TypedSpec().Timeout,
			)
		})
}
