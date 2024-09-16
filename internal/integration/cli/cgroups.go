// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"

	"github.com/siderolabs/talos/internal/integration/base"
)

// CgroupsSuite verifies dmesg command.
type CgroupsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *CgroupsSuite) SuiteName() string {
	return "cli.CgroupsSuite"
}

// TestPresets verifies successful execution.
func (suite *CgroupsSuite) TestPresets() {
	for _, preset := range []string{"cpu", "cpuset", "io", "memory", "process", "swap"} {
		suite.Run(preset, func() {
			suite.RunCLI(
				[]string{
					"cgroups", "--nodes", suite.RandomDiscoveredNodeInternalIP(),
					"--preset", preset,
				},
				base.StdoutShouldMatch(regexp.MustCompile(`apid`)))
		})
	}
}

func init() {
	allSuites = append(allSuites, new(CgroupsSuite))
}
