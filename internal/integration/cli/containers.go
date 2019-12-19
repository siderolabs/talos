// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// ContainersSuite verifies dmesg command
type ContainersSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ContainersSuite) SuiteName() string {
	return "cli.ContainersSuite"
}

// TestContainerd inspects containers via containerd driver.
func (suite *ContainersSuite) TestContainerd() {
	suite.RunOsctl([]string{"containers"},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE`)),
		base.StdoutShouldMatch(regexp.MustCompile(`talos/osd`)),
	)
	suite.RunOsctl([]string{"containers", "-k"},
		base.StdoutShouldMatch(regexp.MustCompile(`kubelet`)),
	)
}

// TestCRI inspects containers via CRI driver.
func (suite *ContainersSuite) TestCRI() {
	suite.RunOsctl([]string{"containers", "-c"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`CRI inspector is supported only for K8s namespace`)))
	suite.RunOsctl([]string{"containers", "-ck"},
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
	)
}

func init() {
	allSuites = append(allSuites, new(ContainersSuite))
}
