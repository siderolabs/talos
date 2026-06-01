// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	cntrconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type ExtensionServiceConfigSuite struct {
	ctest.DefaultSuite
}

func TestExtensionServiceConfigSuite(t *testing.T) {
	suite.Run(t, &ExtensionServiceConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtime.ExtensionServiceConfigController{}))
			},
		},
	})
}

func (suite *ExtensionServiceConfigSuite) TestReconcileExtensionServiceConfig() {
	extensionServiceConfigs := []struct {
		extensionName string
		configFiles   []struct {
			content   string
			mountPath string
		}
		environment []string
	}{
		{
			extensionName: "test-extension-a",
			configFiles: []struct {
				content   string
				mountPath string
			}{
				{
					content:   "test-content-a",
					mountPath: "/etc/test",
				},
			},
		},
		{
			extensionName: "test-extension-b",
			configFiles: []struct {
				content   string
				mountPath string
			}{
				{
					content:   "test-content-b",
					mountPath: "/etc/bar",
				},
				{
					content:   "test-content-c",
					mountPath: "/var/etc/foo",
				},
			},
			environment: []string{
				"FOO=BAR",
			},
		},
	}

	cfgs := xslices.Map(extensionServiceConfigs, func(tt struct {
		extensionName string
		configFiles   []struct {
			content   string
			mountPath string
		}
		environment []string
	},
	) cntrconfig.Document {
		cfg := extensions.NewServicesConfigV1Alpha1()
		cfg.ServiceName = tt.extensionName
		cfg.ServiceConfigFiles = xslices.Map(tt.configFiles, func(config struct {
			content   string
			mountPath string
		},
		) extensions.ConfigFile {
			return extensions.ConfigFile{
				ConfigFileContent:   config.content,
				ConfigFileMountPath: config.mountPath,
			}
		})
		cfg.ServiceEnvironment = tt.environment

		return cfg
	})

	cntr, err := container.New(cfgs...)
	suite.Require().NoError(err)

	machineConfig := config.NewMachineConfig(cntr)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineConfig))

	for _, tt := range extensionServiceConfigs {
		ctest.AssertResource(suite, tt.extensionName, func(config *runtimeres.ExtensionServiceConfig, asrt *assert.Assertions) {
			spec := config.TypedSpec()

			configFileData := xslices.Map(tt.configFiles, func(config struct {
				content   string
				mountPath string
			},
			) runtimeres.ExtensionServiceConfigFile {
				return runtimeres.ExtensionServiceConfigFile{
					Content:   config.content,
					MountPath: config.mountPath,
				}
			})

			suite.Assert().Equal(configFileData, spec.Files)
			suite.Assert().Equal(tt.environment, spec.Environment)
		})
	}

	// test deletion
	cfg := extensions.NewServicesConfigV1Alpha1()
	cfg.ServiceName = "test-extension-a"
	cntr, err = container.New(cfg)
	suite.Require().NoError(err)

	machineConfig = config.NewMachineConfig(cntr)
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), machineConfig.Metadata()))

	ctest.AssertNoResource[*runtimeres.ExtensionServiceConfig](suite, "test-extension-a")
}
