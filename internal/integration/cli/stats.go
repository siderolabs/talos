// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// StatsSuite verifies dmesg command.
type StatsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *StatsSuite) SuiteName() string {
	return "cli.StatsSuite"
}

// TestContainerd inspects stats via containerd driver.
func (suite *StatsSuite) TestContainerd() {
	suite.RunCLI([]string{"stats", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`CPU`)),
		base.StdoutShouldMatch(regexp.MustCompile(`apid`)),
	)
}

// TestCRI inspects stats via CRI driver.
func (suite *StatsSuite) TestCRI() {
	suite.RunCLI([]string{"stats", "-k", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`CPU`)),
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
		base.StdoutShouldMatch(regexp.MustCompile(`k8s.io`)),
	)
}

func init() {
	allSuites = append(allSuites, new(StatsSuite))
}
