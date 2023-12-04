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
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/extensionservicesconfig"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type ExtensionServicesConfigSuite struct {
	ctest.DefaultSuite
}

func TestExtensionServicesConfigSuite(t *testing.T) {
	suite.Run(t, &ExtensionServicesConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtime.ExtensionServicesConfigController{}))
			},
		},
	})
}

func (suite *ExtensionServicesConfigSuite) TestReconcileExtensionServicesConfig() {
	extensionsServiceConfigDoc := extensionservicesconfig.NewExtensionServicesConfigV1Alpha1()
	extensionsServiceConfigDoc.Config = []extensionservicesconfig.ExtensionServiceConfig{
		{
			ExtensionName: "test-extension-a",
			ExtensionServiceConfigFiles: []extensionservicesconfig.ExtensionServiceConfigFile{
				{
					ExtensionContent:   "test-content-a",
					ExtensionMountPath: "/etc/test",
				},
			},
		},
		{
			ExtensionName: "test-extension-b",
			ExtensionServiceConfigFiles: []extensionservicesconfig.ExtensionServiceConfigFile{
				{
					ExtensionContent:   "test-content-b",
					ExtensionMountPath: "/etc/bar",
				},
				{
					ExtensionContent:   "test-content-c",
					ExtensionMountPath: "/var/etc/foo",
				},
			},
		},
	}

	cntr, err := container.New(extensionsServiceConfigDoc)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(cntr)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	for _, tt := range []struct {
		extensionName string
		configFiles   []struct {
			content   string
			mountPath string
		}
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
		},
	} {
		ctest.AssertResource(suite, tt.extensionName, func(config *runtimeres.ExtensionServicesConfig, asrt *assert.Assertions) {
			spec := config.TypedSpec()

			configFileData := xslices.Map(tt.configFiles, func(config struct {
				content   string
				mountPath string
			},
			) runtimeres.ExtensionServicesConfigFile {
				return runtimeres.ExtensionServicesConfigFile{
					Content:   config.content,
					MountPath: config.mountPath,
				}
			})

			suite.Assert().Equal(configFileData, spec.Files)
		})
	}

	// test deletion
	extensionsServiceConfigDoc.Config = extensionsServiceConfigDoc.Config[1:] // remove first extension service config
	cntr, err = container.New(extensionsServiceConfigDoc)
	suite.Require().NoError(err)

	newCfg := config.NewMachineConfig(cntr)
	newCfg.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	ctest.AssertNoResource[*runtimeres.ExtensionServicesConfig](suite, "test-extension-a")
}
