// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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

func (suite *KmsgLogConfigSuite) TestKmsgLogConfigMachineConfig() {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamLoggingKernel, "https://10.0.0.1:3333/logs")

	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.KmsgLogConfigController{
		Cmdline: cmdline,
	}))

	kmsgLogConfig1 := &runtimecfg.KmsgLogV1Alpha1{
		MetaName: "1",
		KmsgLogURL: meta.URL{
			URL: must(url.Parse("https://10.0.0.2:4444/logs")),
		},
		ExtraTags: map[string]string{"cluster": "staging-west"},
	}

	kmsgLogConfig2 := &runtimecfg.KmsgLogV1Alpha1{
		MetaName: "2",
		KmsgLogURL: meta.URL{
			URL: must(url.Parse("https://10.0.0.1:3333/logs")),
		},
		ExtraTags: map[string]string{"node": "worker-1"},
	}

	cfg, err := container.New(kmsgLogConfig1, kmsgLogConfig2)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResources[*runtime.KmsgLogConfig](suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.KmsgLogConfigID},
		func(cfg *runtime.KmsgLogConfig, asrt *assert.Assertions) {
			// the destination from the kernel args is also specified in the machine config, so it is
			// deduplicated, and the extra tags from the machine config are merged into it
			asrt.Equal([]*url.URL{
				must(url.Parse("https://10.0.0.1:3333/logs")),
				must(url.Parse("https://10.0.0.2:4444/logs")),
			}, cfg.TypedSpec().Destinations)
			asrt.Equal([]runtime.KmsgLogDestination{
				{Endpoint: must(url.Parse("https://10.0.0.1:3333/logs")), ExtraTags: map[string]string{"node": "worker-1"}},
				{Endpoint: must(url.Parse("https://10.0.0.2:4444/logs")), ExtraTags: map[string]string{"cluster": "staging-west"}},
			}, cfg.TypedSpec().TaggedDestinations)
		})
}

func (suite *KmsgLogConfigSuite) TestKmsgLogConfigCmdline() {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(constants.KernelParamLoggingKernel, "https://10.0.0.1:3333/logs")

	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.KmsgLogConfigController{
		Cmdline: cmdline,
	}))

	rtestutils.AssertResources[*runtime.KmsgLogConfig](suite.Ctx(), suite.T(), suite.State(), []resource.ID{runtime.KmsgLogConfigID},
		func(cfg *runtime.KmsgLogConfig, asrt *assert.Assertions) {
			asrt.Equal([]*url.URL{must(url.Parse("https://10.0.0.1:3333/logs"))}, cfg.TypedSpec().Destinations)
			asrt.Equal([]runtime.KmsgLogDestination{{Endpoint: must(url.Parse("https://10.0.0.1:3333/logs"))}}, cfg.TypedSpec().TaggedDestinations)
		})
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
