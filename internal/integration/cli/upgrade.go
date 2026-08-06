// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

const incompatibleInstallerImage = "ghcr.io/siderolabs/installer:v1.12.0"

// UpgradeSuite tests upgrade command failure reporting.
type UpgradeSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *UpgradeSuite) SuiteName() string {
	return "cli.UpgradeSuite"
}

// TestIncompatibleMachineConfig verifies that installer diagnostics survive a failed downgrade.
func (suite *UpgradeSuite) TestIncompatibleMachineConfig() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Cluster == nil {
		suite.T().Skip("requires a disposable provisioned cluster")
	}

	if suite.Airgapped {
		suite.T().Skip("requires pulling the old installer image")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	config, _ := suite.RunCLI([]string{"get", "--nodes", node, "mc", "v1alpha1", "--output", "jsonpath={.spec}"})
	suite.Require().Contains(config, "kind: DiscoveryServiceConfig", "downgrade must be guaranteed to fail before touching disk")

	suite.RunCLI(
		[]string{
			"upgrade",
			"--nodes", node,
			"--image", incompatibleInstallerImage,
			"--namespace", "inmem",
			"--no-reboot",
		},
		base.StdoutEmpty(),
		base.ShouldFail(),
		base.StderrMatchFunc(func(stderr string) error {
			for _, expected := range []string{
				"error loading machine configuration",
				"not registered",
				fmt.Sprintf("%s: upgrade failed with exit code 1", node),
			} {
				if !strings.Contains(stderr, expected) {
					return fmt.Errorf("expected upgrade stderr to contain %q", expected)
				}
			}

			return nil
		}),
	)
}

func init() {
	allSuites = append(allSuites, new(UpgradeSuite))
}
