// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	siderolinkctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	siderolinkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

type ConfigSuite struct {
	ctest.DefaultSuite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, &ConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&siderolinkctrl.ConfigController{}))
			},
			Timeout: time.Second,
		},
	})
}

func (suite *ConfigSuite) TestConfig() {
	rtestutils.AssertNoResource[*siderolink.Config](suite.Ctx(), suite.T(), suite.State(), siderolink.ConfigID)

	siderolinkConfig := &siderolinkcfg.ConfigV1Alpha1{
		APIUrlConfig: meta.URL{
			URL: must.Value(url.Parse("https://api.sidero.dev"))(suite.T()),
		},
	}

	cfg, err := container.New(siderolinkConfig)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{siderolink.ConfigID},
		func(c *siderolink.Config, assert *assert.Assertions) {
			assert.Equal("https://api.sidero.dev", c.TypedSpec().APIEndpoint)
		})
}

func (suite *ConfigSuite) TestConfigTunnel() {
	rtestutils.AssertNoResource[*siderolink.Config](suite.Ctx(), suite.T(), suite.State(), siderolink.ConfigID)

	siderolinkConfig := &siderolinkcfg.ConfigV1Alpha1{
		APIUrlConfig: meta.URL{
			URL: must.Value(url.Parse("https://api.sidero.dev?grpc_tunnel=true"))(suite.T()),
		},
	}

	cfg, err := container.New(siderolinkConfig)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{siderolink.ConfigID},
		func(c *siderolink.Config, assert *assert.Assertions) {
			assert.Equal("https://api.sidero.dev?grpc_tunnel=true", c.TypedSpec().APIEndpoint)
			assert.True(c.TypedSpec().Tunnel)
		})
}
