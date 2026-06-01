// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/integration/base"
)

var (
	//go:embed testdata/inject/talosconfig-input.yaml
	inputManifests []byte

	//go:embed testdata/inject/talosconfig-expected.yaml
	expectedManifests []byte
)

// InjectSuite verifies inject command.
type InjectSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *InjectSuite) SuiteName() string {
	return "cli.InjectSuite"
}

// TestServiceAccount tests inject serviceaccount command.
func (suite *InjectSuite) TestServiceAccount() {
	suite.testServiceAccount(inputManifests)
}

// TestServiceAccountAlreadyInjectedNoChange tests inject serviceaccount command when the input manifest is already injected,
// makes sure that it stays the same.
func (suite *InjectSuite) TestServiceAccountAlreadyInjectedNoChange() {
	suite.testServiceAccount(expectedManifests)
}

func (suite *InjectSuite) testServiceAccount(input []byte) {
	expectedDocs, err := yamlDocs(expectedManifests)
	suite.Require().NoError(err)

	tempDir := suite.T().TempDir()

	inputPath := filepath.Join(tempDir, "input.yaml")

	err = os.WriteFile(inputPath, input, 0o644)
	suite.Require().NoError(err)

	stdout, _ := suite.RunCLI([]string{"inject", "serviceaccount", "-f", inputPath, "--roles", "os:reader,os:admin"})

	stdoutDocs, err := yamlDocs([]byte(stdout))
	suite.Require().NoError(err)

	suite.Assert().Equal(expectedDocs, stdoutDocs, "inject serviceaccount output did not match expected output")
}

func yamlDocs(input []byte) ([]map[string]any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(input))

	var docs []map[string]any

	for {
		var doc map[string]any

		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("document decode failed: %w", err)
		}

		docs = append(docs, doc)
	}

	return docs, nil
}

func init() {
	allSuites = append(allSuites, new(InjectSuite))
}
