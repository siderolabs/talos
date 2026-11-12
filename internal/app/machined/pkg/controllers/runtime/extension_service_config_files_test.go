// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package runtime_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/opendir"
)

type ExtensionServiceConfigFilesSuite struct {
	ctest.DefaultSuite

	extensionsConfigDir string
}

func TestExtensionServiceConfigFilesSuite(t *testing.T) {
	extensionsConfigDir := t.TempDir()

	root := &xfs.UnixRoot{FS: opendir.NewFromPath(extensionsConfigDir)}

	suite.Run(t, &ExtensionServiceConfigFilesSuite{
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(root.OpenFS())
			suite.Require().NoError(suite.Runtime().RegisterController(&runtime.ExtensionServiceConfigFilesController{
				ExtensionsConfigRoot: root,
			}))
		},
		AfterTearDown: func(*ctest.DefaultSuite) {
			root.Close() //nolint:errcheck
		},
		extensionsConfigDir: extensionsConfigDir,
	})
}

func (suite *ExtensionServiceConfigFilesSuite) TestReconcileExtensionServiceConfigFiles() {
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
		extensionServiceConfigFiles := runtimeres.NewExtensionServiceConfigSpec(runtimeres.NamespaceName, tt.extensionName)
		extensionServiceConfigFiles.TypedSpec().Files = xslices.Map(tt.configFiles, func(config struct {
			content   string
			mountPath string
		},
		) runtimeres.ExtensionServiceConfigFile {
			return runtimeres.ExtensionServiceConfigFile{
				Content:   config.content,
				MountPath: config.mountPath,
			}
		})

		suite.Require().NoError(suite.State().Create(suite.Ctx(), extensionServiceConfigFiles))

		ctest.AssertResource(
			suite, tt.extensionName,
			func(status *runtimeres.ExtensionServiceConfigStatus, asrt *assert.Assertions) {
				asrt.Equal(extensionServiceConfigFiles.Metadata().Version().String(), status.TypedSpec().SpecVersion)
			},
		)

		for _, file := range tt.configFiles {
			content, err := os.ReadFile(filepath.Join(suite.extensionsConfigDir, tt.extensionName, file.mountPath))
			suite.Require().NoError(err)

			suite.Assert().Equal(file.content, string(content))
		}
	}

	// create a directory and file manually in the extensions config directory
	// ensure that the controller deletes the manually created directory/file
	// also ensure that an update doesn't update existing files timestamp
	suite.Assert().NoError(os.Mkdir(filepath.Join(suite.extensionsConfigDir, "test"), 0o755))
	suite.Assert().NoError(os.WriteFile(filepath.Join(suite.extensionsConfigDir, "test", "testdata"), []byte("{}"), 0o644))

	extensionAConfigFileInfo, err := os.Stat(filepath.Join(suite.extensionsConfigDir, "test-extension-a", "etc", "test"))
	suite.Assert().NoError(err)

	// delete test-extension-b resource
	suite.Assert().NoError(suite.State().Destroy(suite.Ctx(), runtimeres.NewExtensionServiceConfigSpec(runtimeres.NamespaceName, "test-extension-b").Metadata()))
	ctest.AssertNoResource[*runtimeres.ExtensionServiceConfigStatus](suite, "test-extension-b")

	suite.Assert().NoFileExists(filepath.Join(suite.extensionsConfigDir, "test", "testdata"))
	suite.Assert().NoDirExists(filepath.Join(suite.extensionsConfigDir, "test"))
	suite.Assert().NoFileExists(filepath.Join(suite.extensionsConfigDir, "test-extension-b", "etc", "bar"))
	suite.Assert().NoFileExists(filepath.Join(suite.extensionsConfigDir, "test-extension-b", "var", "etc", "foo"))
	suite.Assert().NoDirExists(filepath.Join(suite.extensionsConfigDir, "test-extension-b"))

	extensionAConfigFileInfoAfterUpdate, err := os.Stat(filepath.Join(suite.extensionsConfigDir, "test-extension-a", "etc", "test"))
	suite.Require().NoError(err)

	suite.Assert().Equal(extensionAConfigFileInfo.ModTime(), extensionAConfigFileInfoAfterUpdate.ModTime())

	// a mount path which turns a config file into a directory replaces the file
	extensionA, err := safe.StateUpdateWithConflicts(suite.Ctx(), suite.State(), runtimeres.NewExtensionServiceConfigSpec(runtimeres.NamespaceName, "test-extension-a").Metadata(),
		func(res *runtimeres.ExtensionServiceConfig) error {
			res.TypedSpec().Files = []runtimeres.ExtensionServiceConfigFile{{Content: "test-content-d", MountPath: "/etc/test/inner"}}

			return nil
		},
	)
	suite.Require().NoError(err)

	ctest.AssertResource(
		suite, "test-extension-a",
		func(status *runtimeres.ExtensionServiceConfigStatus, asrt *assert.Assertions) {
			asrt.Equal(extensionA.Metadata().Version().String(), status.TypedSpec().SpecVersion)
		},
	)

	content, err := os.ReadFile(filepath.Join(suite.extensionsConfigDir, "test-extension-a", "etc", "test", "inner"))
	suite.Require().NoError(err)
	suite.Assert().Equal("test-content-d", string(content))
}
