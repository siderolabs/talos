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

// ContainersSuite verifies dmesg command.
type ContainersSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ContainersSuite) SuiteName() string {
	return "cli.ContainersSuite"
}

// TestContainerd inspects containers via containerd driver.
func (suite *ContainersSuite) TestContainerd() {
	suite.RunCLI([]string{"containers", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE`)),
		base.StdoutShouldMatch(regexp.MustCompile(`apid`)),
	)
}

// TestCRI inspects containers via CRI driver.
func (suite *ContainersSuite) TestCRI() {
	suite.RunCLI([]string{"containers", "-k", "--nodes", suite.RandomDiscoveredNode(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
	)
}

func init() {
	allSuites = append(allSuites, new(ContainersSuite))
}
