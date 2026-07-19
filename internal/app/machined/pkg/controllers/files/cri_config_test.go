// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

type CRIConfigSuite struct {
	ctest.DefaultSuite

	root *xfs.OSRoot
}

func TestCRIConfigSuite(t *testing.T) {
	t.Parallel()

	root := &xfs.OSRoot{Shadow: t.TempDir()}

	for _, path := range []string{
		filepath.Join(root.Shadow, constants.CRIConfdPath),
		filepath.Join(root.Shadow, constants.CRIConfdPath, "hosts"),
	} {
		assert.NoError(t, os.MkdirAll(path, 0o755))
	}

	assert.NoError(t, os.WriteFile(
		filepath.Join(root.Shadow, constants.CRIConfdPath, "00-base.part"),
		[]byte("version = 2\n[base]\nenabled = true\n"),
		0o600,
	))
	assert.NoError(t, os.WriteFile(
		filepath.Join(root.Shadow, constants.CRIConfdPath, "15-extension.part"),
		[]byte("[extension]\nenabled = true\n"),
		0o600,
	))
	assert.NoError(t, os.WriteFile(
		filepath.Join(root.Shadow, constants.CRICustomizationConfigPart),
		[]byte("[physical_collision]\nenabled = true\n"),
		0o600,
	))

	suite.Run(t, &CRIConfigSuite{
		root: root,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.CRIConfigController{EtcRoot: root}))
			},
		},
	})
}

func (suite *CRIConfigSuite) TestMergedConfigAndHosts() {
	cfg := crires.NewRegistriesConfig()
	cfg.TypedSpec().RegistryMirrors = map[string]*crires.RegistryMirrorConfig{
		"docker.io": {
			MirrorEndpoints: []crires.RegistryEndpointConfig{{EndpointEndpoint: "https://mirror.example.com"}},
		},
	}

	qualified := crires.NewCustomizationConfig("example.com/qualified")
	qualified.TypedSpec().Content = "[qualified]\nenabled = true\n"

	suite.Create(cfg)
	ctest.AssertResource(suite, constants.CRIConfig, func(res *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := string(res.TypedSpec().Contents)

		asrt.Contains(contents, "## file /etc/cri/conf.d/00-base.part")
		asrt.Contains(contents, "## in-memory registries")
		asrt.Contains(contents, "## file /etc/cri/conf.d/20-customization.part")
		asrt.Contains(contents, "[physical_collision]")
	})

	suite.Create(qualified)

	basePart := "## file /etc/cri/conf.d/00-base.part"
	registryPart := "## in-memory registries"
	extensionPart := "## file /etc/cri/conf.d/15-extension.part"
	customizationPart := "## file /etc/cri/conf.d/20-customization.part"
	qualifiedPart := "## in-memory customization \"example.com/qualified\""

	ctest.AssertResource(suite, constants.CRIConfig, func(res *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := res.TypedSpec().Contents

		indices := []int{
			bytes.Index(contents, []byte(basePart)),
			bytes.Index(contents, []byte(registryPart)),
			bytes.Index(contents, []byte(extensionPart)),
			bytes.Index(contents, []byte(customizationPart)),
			bytes.Index(contents, []byte(qualifiedPart)),
		}

		for i, index := range indices {
			asrt.NotEqual(-1, index, "missing configuration part %d", i)
		}

		for i := 1; i < len(indices); i++ {
			asrt.Less(indices[i-1], indices[i])
		}

		asrt.Contains(string(contents), "[base]")
		asrt.Contains(string(contents), "[extension]")
		asrt.Contains(string(contents), "[physical_collision]")
		asrt.Contains(string(contents), "[qualified]")
		asrt.NotContains(string(contents), "## 01-registries.part")
		asrt.Empty(res.Metadata().Annotations().Raw())
	})
	ctest.AssertNoResource[*files.EtcFileSpec](suite, "cri/conf.d/01-registries.part")

	hosts, err := xfs.ReadFile(suite.root, filepath.Join(constants.CRIConfdPath, "hosts", "docker.io", "hosts.toml"))
	suite.Require().NoError(err)
	suite.Contains(string(hosts), "https://mirror.example.com")

	suite.Destroy(qualified)

	replacement := crires.NewCustomizationConfig("replacement")
	replacement.TypedSpec().Content = "[replacement]\nenabled = true\n"
	suite.Create(replacement)

	ctest.AssertResource(suite, constants.CRIConfig, func(res *files.EtcFileSpec, asrt *assert.Assertions) {
		contents := string(res.TypedSpec().Contents)

		asrt.Contains(contents, "[replacement]")
		asrt.Contains(contents, "[physical_collision]")
		asrt.NotContains(contents, "[qualified]")
	})
}
