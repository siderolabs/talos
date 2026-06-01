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

type EnvironmentSuite struct {
	ctest.DefaultSuite
}

func TestEnvironmentSuite(t *testing.T) {
	suite.Run(t, new(EnvironmentSuite))
}

func (suite *EnvironmentSuite) TestEnvironmentNone() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.EnvironmentController{}))

	rtestutils.AssertResource[*runtime.Environment](suite.Ctx(), suite.T(), suite.State(), "machined",
		func(r *runtime.Environment, asrt *assert.Assertions) {
			asrt.NotEmpty(r.TypedSpec().Variables)
		})
}

func (suite *EnvironmentSuite) TestEnvironmentMachineConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.EnvironmentController{}))

	cfg, err := container.New(&runtimecfg.EnvironmentV1Alpha1{
		EnvironmentVariables: map[string]string{
			"TEST": "value",
		},
	})
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResources[*runtime.Environment](suite.Ctx(), suite.T(), suite.State(), []resource.ID{"machined"},
		func(cfg *runtime.Environment, asrt *assert.Assertions) {
			asrt.Contains(
				cfg.TypedSpec().Variables,
				"TEST=value",
			)
		})
}
