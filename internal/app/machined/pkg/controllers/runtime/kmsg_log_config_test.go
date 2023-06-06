// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type KmsgLogConfigSuite struct {
	ctest.DefaultSuite
}

func TestKmsgLogConfigSuite(t *testing.T) {
	suite.Run(t, new(KmsgLogConfigSuite))
}

func (suite *KmsgLogConfigSuite) TestKmsgLogConfigNone() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.KmsgLogConfigController{}))

	rtestutils.AssertNoResource[*runtime.KmsgLogConfig](suite.Ctx(), suite.T(), suite.State(), runtime.KmsgLogConfigID)
}

func (suite *KmsgLogConfigSuite) TestKmsgLogConfigCmdline() {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamLoggingKernel, "https://10.0.0.1:3333/logs")

	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.KmsgLogConfigController{
		Cmdline: cmdline,
	}))

	rtestutils.AssertResources[*runtime.KmsgLogConfig](suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.KmsgLogConfigID},
		func(cfg *runtime.KmsgLogConfig, asrt *assert.Assertions) {
			asrt.Equal(
				[]string{"https://10.0.0.1:3333/logs"},
				slices.Map(cfg.TypedSpec().Destinations, func(u *url.URL) string { return u.String() }),
			)
		})
}
