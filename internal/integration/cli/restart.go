// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
)

// RestartSuite verifies dmesg command
type RestartSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *RestartSuite) SuiteName() string {
	return "cli.RestartSuite"
}

// TestSystem restarts system containerd process.
func (suite *RestartSuite) TestSystem() {
	suite.RunOsctl([]string{"restart", "trustd"},
		base.StdoutEmpty())

	time.Sleep(50 * time.Millisecond)

	suite.RunAndWaitForMatch([]string{"containers"}, regexp.MustCompile(`trustd`), 30*time.Second)
}

// TestKubernetes restarts K8s container.
func (suite *RestartSuite) TestK8s() {
	suite.RunOsctl([]string{"restart", "-k", "kubelet"},
		base.StdoutEmpty())

	time.Sleep(50 * time.Millisecond)

	suite.RunAndWaitForMatch([]string{"containers", "-k"}, regexp.MustCompile(`\s+kubelet\s+`), 30*time.Second)
}

func init() {
	allSuites = append(allSuites, new(RestartSuite))
}
