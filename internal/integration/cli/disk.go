// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"github.com/talos-systems/talos/internal/integration/base"
)

// DisksSuite verifies dmesg command.
type DisksSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *DisksSuite) SuiteName() string {
	return "cli.DisksSuite"
}

// TestSuccess runs comand with success.
func (suite *DisksSuite) TestSuccess() {
	suite.RunCLI([]string{"disks", "--nodes", suite.RandomDiscoveredNode()})
}

func init() {
	allSuites = append(allSuites, new(DisksSuite))
}
