// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
)

// TalosconfigSuite verifies dmesg command.
type TalosconfigSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *TalosconfigSuite) SuiteName() string {
	return "cli.TalosconfigSuite"
}

// TestList checks how talosctl config merge.
func (suite *TalosconfigSuite) TestList() {
	suite.RunCLI([]string{"config", "contexts"},
		base.StdoutShouldMatch(regexp.MustCompile(`CURRENT`)))
}

func init() {
	allSuites = append(allSuites, new(TalosconfigSuite))
}
