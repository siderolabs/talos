// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"os"
	"regexp"
	"strings"

	"github.com/talos-systems/talos/internal/integration/base"
)

// MountsSuite verifies dmesg command.
type MountsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *MountsSuite) SuiteName() string {
	return "cli.MountsSuite"
}

// TestSuccess verifies successful execution.
func (suite *MountsSuite) TestSuccess() {
	suite.RunCLI([]string{"mounts", "--nodes", suite.RandomDiscoveredNode()},
		base.StdoutShouldMatch(regexp.MustCompile(`(?s)FILESYSTEM.*`)))
}

// TestUserDisksMounted verifies user disk mounts created.
func (suite *MountsSuite) TestUserDisksMounted() {
	paths := os.Getenv("USER_DISKS_MOUNTS")

	if paths == "" {
		return
	}

	for _, path := range strings.Split(paths, ",") {
		suite.RunCLI([]string{"mounts", "--nodes", suite.RandomDiscoveredNode()},
			base.StdoutShouldMatch(regexp.MustCompile(path)))
	}
}

func init() {
	allSuites = append(allSuites, new(MountsSuite))
}
