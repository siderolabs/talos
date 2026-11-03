// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/images"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

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

	versionOut, _ := suite.RunCLI([]string{"version", "--json", "--nodes", node})

	var nodeVersion machineapi.Version

	suite.Require().NoError(protojson.Unmarshal([]byte(versionOut), &nodeVersion))

	nodeTag, err := semver.ParseTolerant(nodeVersion.GetVersion().GetTag())
	suite.Require().NoError(err)

	factoryInstaller := func(version string) string {
		return images.NewInstallerImage("", "metal", images.DefaultInstallerImageSchematic, version)
	}

	config, _ := suite.RunCLI([]string{"get", "--nodes", node, "mc", "v1alpha1", "--output", "jsonpath={.spec}"})

	for _, tt := range []struct {
		version      string
		installer    string // installer image repository, image factory is used when empty
		configMarker string
		exitCode     int
	}{
		{
			version:      "v1.12.0",
			installer:    "ghcr.io/siderolabs/installer",
			configMarker: "kind: ImageVerificationConfig",
			exitCode:     1, // predates classified installer exit codes
		},
		{
			version:      "v1.14.2",
			configMarker: "trustdCheckExtKeyUsage: true",
			exitCode:     constants.ExitInvalidInput,
		},
	} {
		suite.Run(tt.version, func() {
			if installerTag := semver.MustParse(strings.TrimPrefix(tt.version, "v")); installerTag.GE(nodeTag) {
				suite.T().Skipf("installer %s is not older than node version %s", tt.version, nodeTag)
			}

			if !strings.Contains(config, tt.configMarker) {
				suite.T().Skipf("machine config lacks %q, installer %s might accept it", tt.configMarker, tt.version)
			}

			image := factoryInstaller(tt.version)
			if tt.installer != "" {
				image = tt.installer + ":" + tt.version
			}

			suite.RunCLI(
				[]string{
					"upgrade",
					"--nodes", node,
					"--image", image,
					"--namespace", "inmem",
					"--no-reboot",
				},
				base.StdoutEmpty(),
				base.ShouldFail(),
				base.StderrMatchFunc(func(stderr string) error {
					for _, expected := range []string{
						"error loading machine configuration",
						fmt.Sprintf("%s: upgrade failed with exit code %d", node, tt.exitCode),
					} {
						if !strings.Contains(stderr, expected) {
							return fmt.Errorf("expected upgrade stderr to contain %q", expected)
						}
					}

					return nil
				}),
			)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(UpgradeSuite))
}
