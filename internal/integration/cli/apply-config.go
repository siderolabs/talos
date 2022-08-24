// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ApplyConfigSuite verifies dmesg command.
type ApplyConfigSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ApplyConfigSuite) SuiteName() string {
	return "cli.ApplyConfigSuite"
}

// TestApplyWithPatch verifies that .
func (suite *ApplyConfigSuite) TestApplyWithPatch() {
	patch := `---
machine:
  network:
    interfaces:
      - interface: dummy-ap-patch
        dummy: true`

	node := suite.RandomDiscoveredNodeInternalIP()

	patchPath := filepath.Join(suite.T().TempDir(), "patch.yaml")

	suite.Require().NoError(os.WriteFile(patchPath, []byte(patch), 0o777))

	data, _ := suite.RunCLI([]string{"read", "--nodes", node, "/system/state/config.yaml"})

	configPath := filepath.Join(suite.T().TempDir(), "config.yaml")

	suite.Require().NoError(os.WriteFile(configPath, []byte(data), 0o777))

	suite.RunCLI([]string{"apply-config", "--nodes", node, "--config-patch", fmt.Sprintf("@%s", patchPath), "-f", configPath})

	suite.RunCLI([]string{"get", "--nodes", node, "links"},
		base.StdoutShouldMatch(regexp.MustCompile("dummy-ap-patch")),
		base.WithRetry(retry.Constant(15*time.Second, retry.WithUnits(time.Second))),
	)
}

func init() {
	allSuites = append(allSuites, new(ApplyConfigSuite))
}
