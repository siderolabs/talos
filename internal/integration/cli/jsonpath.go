// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"time"

	"github.com/siderolabs/go-retry/retry"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// JSONPathSuite verifies dmesg command.
type JSONPathSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *JSONPathSuite) SuiteName() string {
	return "cli.JSONPathSuite"
}

// TestGetScalarPropertyWithJSONPath verifies that the jsonpath filter to the get command can return scalar data.
func (suite *JSONPathSuite) TestGetScalarPropertyWithJSONPath() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.RunCLI([]string{"get", "--nodes", node, "etcfilestatus", "--output", `jsonpath='{.metadata.namespace}'`},
		base.StdoutShouldMatch(regexp.MustCompile("files")),
		base.WithRetry(retry.Constant(15*time.Second, retry.WithUnits(time.Second))),
	)
}

// TestGetWithJSONPathWildcard verifies that the jsonpath filter to the get command accepts a wildcard operator.
// It is handy when 'get' requests a list of resources.
func (suite *JSONPathSuite) TestGetWithJSONPathWildcard() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.RunCLI([]string{"get", "--nodes", node, "manifests", "--output", `jsonpath='{.spec[*].metadata.name}'`},
		base.StdoutShouldMatch(regexp.MustCompile("kube-proxy")),
		base.StdoutShouldMatch(regexp.MustCompile("coredns")),
		base.StdoutShouldMatch(regexp.MustCompile("kube-dns")),
		base.StdoutShouldMatch(regexp.MustCompile("kubeconfig-in-cluster")),
		base.WithRetry(retry.Constant(15*time.Second, retry.WithUnits(time.Second))),
	)
}

// TestGetComplexPropertyWithJSONPath verifies that the jsonpath filter to the get command can return JSON.
func (suite *JSONPathSuite) TestGetComplexPropertyWithJSONPath() {
	node := suite.RandomDiscoveredNodeInternalIP()

	const jsonMetadataRegex = `\{\s*"created":\s".*",\s*"id":\s".*",\s*\s*"namespace":\s".*",\s*"owner":\s".*",\s*"phase":\s".*",\s*"type":\s".*",\s*"updated":\s".*",\s*"version":\s\d\n\}`

	suite.RunCLI([]string{"get", "--nodes", node, "etcfilestatus", "--output", `jsonpath='{.metadata}'`},
		base.StdoutShouldMatch(regexp.MustCompile(jsonMetadataRegex)),
		base.WithRetry(retry.Constant(15*time.Second, retry.WithUnits(time.Second))),
	)
}

func init() {
	allSuites = append(allSuites, new(JSONPathSuite))
}
