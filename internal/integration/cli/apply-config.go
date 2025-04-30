// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/integration/base"
)

// ApplyConfigSuite verifies dmesg command.
type ApplyConfigSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ApplyConfigSuite) SuiteName() string {
	return "cli.ApplyConfigSuite"
}

//go:embed testdata/patches/dummy-ap.yaml
var dummyAPPatch []byte

//go:embed testdata/patches/delete-dummy-ap.yaml
var deleteDummyAPPatch []byte

// TestApplyWithPatch verifies that .
func (suite *ApplyConfigSuite) TestApplyWithPatch() {
	tmpDir := suite.T().TempDir()

	node := suite.RandomDiscoveredNodeInternalIP()

	data, _ := suite.RunCLI([]string{"get", "--nodes", node, "mc", "v1alpha1", "-o", "jsonpath={.spec}"})

	configPath := filepath.Join(tmpDir, "config.yaml")
	suite.Require().NoError(os.WriteFile(configPath, []byte(data), 0o777))

	patchPath := filepath.Join(tmpDir, "patch.yaml")
	suite.Require().NoError(os.WriteFile(patchPath, dummyAPPatch, 0o777))

	suite.RunCLI([]string{"apply-config", "--nodes", node, "--config-patch", "@" + patchPath, "-f", configPath},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile("Applied configuration without a reboot")),
	)

	suite.RunCLI([]string{"get", "--nodes", node, "links"},
		base.StdoutShouldMatch(regexp.MustCompile("dummy-ap-patch")),
		base.WithRetry(retry.Constant(15*time.Second, retry.WithUnits(time.Second))),
	)

	// now delete the dummy-ap-patch
	data, _ = suite.RunCLI([]string{"get", "--nodes", node, "mc", "v1alpha1", "-o", "jsonpath={.spec}"})
	suite.Require().NoError(os.WriteFile(configPath, []byte(data), 0o777))

	suite.Require().NoError(os.WriteFile(patchPath, deleteDummyAPPatch, 0o777))

	suite.RunCLI([]string{"apply-config", "--nodes", node, "--config-patch", "@" + patchPath, "-f", configPath},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile("Applied configuration without a reboot")),
	)

	suite.RunCLI([]string{"get", "--nodes", node, "links"},
		base.StdoutShouldNotMatch(regexp.MustCompile("dummy-ap-patch")),
		base.WithRetry(retry.Constant(15*time.Second, retry.WithUnits(time.Second))),
	)
}

func init() {
	allSuites = append(allSuites, new(ApplyConfigSuite))
}
