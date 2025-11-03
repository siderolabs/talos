// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/integration/base"
)

// MachineConfigSuite verifies machineconfig command.
type MachineConfigSuite struct {
	base.CLISuite

	savedCwd string
}

// SuiteName ...
func (suite *MachineConfigSuite) SuiteName() string {
	return "cli.MachineConfigSuite"
}

// SetupTest ...
func (suite *MachineConfigSuite) SetupTest() {
	var err error

	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)
}

// TearDownTest ...
func (suite *MachineConfigSuite) TearDownTest() {
	if suite.savedCwd != "" {
		suite.Require().NoError(os.Chdir(suite.savedCwd))
	}
}

// TestGen tests the gen subcommand.
// Identical to GenSuite.TestGenConfigToStdoutControlPlane
// The remaining functionality is checked for `talosctl gen config` in GenSuite.
func (suite *MachineConfigSuite) TestGen() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "controlplane",
		"--output", "-",
	}, base.StderrNotEmpty(), base.StdoutMatchFunc(func(output string) error {
		expected := "type: controlplane"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stdout does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

// TestPatchPrintStdout tests the patch subcommand with output set to stdout
// with multiple patches from the command line and from file.
func (suite *MachineConfigSuite) TestPatchPrintStdout() {
	patch1, err := yaml.Marshal(map[string]any{
		"cluster": map[string]any{
			"clusterName": "replaced",
		},
	})
	suite.Require().NoError(err)

	patch2, err := yaml.Marshal(map[string]any{
		"cluster": map[string]any{
			"controlPlane": map[string]any{
				"endpoint": "https://replaced",
			},
		},
	})
	suite.Require().NoError(err)

	patch2Path := filepath.Join(suite.T().TempDir(), "patch2.json")

	err = os.WriteFile(patch2Path, patch2, 0o644)
	suite.Require().NoError(err)

	mc := filepath.Join(suite.T().TempDir(), "input.yaml")

	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "controlplane",
		"--output", mc,
	},
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)

	suite.RunCLI([]string{
		"machineconfig", "patch", mc, "--patch", string(patch1), "-p", "@" + patch2Path,
	}, base.StdoutMatchFunc(func(output string) error {
		var matchErr *multierror.Error

		if !strings.Contains(output, "clusterName: replaced") {
			matchErr = multierror.Append(matchErr, errors.New("clusterName not replaced"))
		}

		if !strings.Contains(output, "endpoint: https://replaced") {
			matchErr = multierror.Append(matchErr, errors.New("endpoint not replaced"))
		}

		return matchErr.ErrorOrNil()
	}))
}

// TestPatchWriteToFile tests the patch subcommand with output set to a file.
func (suite *MachineConfigSuite) TestPatchWriteToFile() {
	patch1, err := yaml.Marshal(map[string]any{
		"cluster": map[string]any{
			"clusterName": "replaced",
		},
	})
	suite.Require().NoError(err)

	mc := filepath.Join(suite.T().TempDir(), "input2.yaml")

	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "controlplane",
		"--output", mc,
	},
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)

	outputFile := filepath.Join(suite.T().TempDir(), "inner", "output.yaml")

	suite.RunCLI([]string{
		"machineconfig", "patch", mc, "--patch", string(patch1), "--output", outputFile,
	}, base.StdoutEmpty())

	outputBytes, err := os.ReadFile(outputFile)
	suite.Assert().NoError(err)

	suite.Assert().Contains(string(outputBytes), "clusterName: replaced")
}

func init() {
	allSuites = append(allSuites, new(MachineConfigSuite))
}
