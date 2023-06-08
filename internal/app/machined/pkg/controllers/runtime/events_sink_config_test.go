// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type EventsSinkConfigSuite struct {
	ctest.DefaultSuite
}

func TestEventsSinkConfigSuite(t *testing.T) {
	suite.Run(t, new(EventsSinkConfigSuite))
}

func (suite *EventsSinkConfigSuite) TestEventSinkConfigNone() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.EventsSinkConfigController{}))

	rtestutils.AssertNoResource[*runtime.EventSinkConfig](suite.Ctx(), suite.T(), suite.State(), runtime.EventSinkConfigID)
}

func (suite *EventsSinkConfigSuite) TestEventSinkConfigCmdline() {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamEventsSink, "10.0.0.1:3333")

	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.EventsSinkConfigController{
		Cmdline: cmdline,
	}))

	rtestutils.AssertResources[*runtime.EventSinkConfig](suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.EventSinkConfigID},
		func(cfg *runtime.EventSinkConfig, asrt *assert.Assertions) {
			asrt.Equal(
				"10.0.0.1:3333",
				cfg.TypedSpec().Endpoint,
			)
		})
}
