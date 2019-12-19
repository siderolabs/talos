// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// MountsSuite verifies dmesg command
type MountsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *MountsSuite) SuiteName() string {
	return "cli.MountsSuite"
}

// TestSuccess verifies successful execution.
func (suite *MountsSuite) TestSuccess() {
	suite.RunOsctl([]string{"mounts"},
		base.StdoutShouldMatch(regexp.MustCompile(`FILESYSTEM`)))
}

func init() {
	allSuites = append(allSuites, new(MountsSuite))
}
