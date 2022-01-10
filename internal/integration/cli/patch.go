// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// PatchSuite verifies dmesg command.
type PatchSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *PatchSuite) SuiteName() string {
	return "cli.PatchSuite"
}

// TestSuccess successful run.
func (suite *PatchSuite) TestSuccess() {
	node := suite.RandomDiscoveredNode(machine.TypeControlPlane)

	patch := []map[string]interface{}{
		{
			"op":   "replace",
			"path": "/cluster/proxy",
			"value": map[string]interface{}{
				"image": fmt.Sprintf("%s:v%s", constants.KubeProxyImage, constants.DefaultKubernetesVersion),
			},
		},
	}

	data, err := json.Marshal(patch)
	suite.Require().NoError(err)

	suite.RunCLI([]string{"patch", "--nodes", node, "--patch", string(data), "machineconfig", "--mode=no-reboot"})
}

// TestError runs comand with error.
func (suite *PatchSuite) TestError() {
	node := suite.RandomDiscoveredNode(machine.TypeControlPlane)

	patch := []map[string]interface{}{
		{
			"op":   "crash",
			"path": "/cluster/proxy",
			"value": map[string]interface{}{
				"image": fmt.Sprintf("%s:v%s", constants.KubeProxyImage, constants.DefaultKubernetesVersion),
			},
		},
	}

	data, err := json.Marshal(patch)
	suite.Require().NoError(err)

	suite.RunCLI([]string{"patch", "--nodes", node, "--patch", string(data), "machineconfig"},
		base.StdoutEmpty(), base.ShouldFail())
	suite.RunCLI([]string{"patch", "--nodes", node, "--patch", string(data), "machineconfig", "v1alpha2"},
		base.StdoutEmpty(), base.ShouldFail())
	suite.RunCLI([]string{"patch", "--nodes", node, "--patch-file", "/nnnope", "machineconfig"},
		base.StdoutEmpty(), base.ShouldFail())
	suite.RunCLI([]string{"patch", "--nodes", node, "--patch", "it's not even a json", "machineconfig"},
		base.StdoutEmpty(), base.ShouldFail())
}

func init() {
	allSuites = append(allSuites, new(PatchSuite))
}
