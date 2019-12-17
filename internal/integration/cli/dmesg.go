// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"github.com/talos-systems/talos/internal/integration/base"
)

// DmesgSuite verifies dmesg command
type DmesgSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *DmesgSuite) SuiteName() string {
	return "cli.DmesgSuite"
}

// TestHasOutput verifies that dmesg is displayed.
func (suite *DmesgSuite) TestHasOutput() {
	suite.RunOsctl([]string{"dmesg"}) // default checks for stdout not empty
}

func init() {
	allSuites = append(allSuites, new(DmesgSuite))
}
