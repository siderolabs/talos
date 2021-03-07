// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"github.com/talos-systems/talos/internal/integration/base"
)

// ImagesSuite verifies the images command.
type ImagesSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ImagesSuite) SuiteName() string {
	return "cli.ImagesSuite"
}

// TestSuccess verifies successful execution.
func (suite *ImagesSuite) TestSuccess() {
	suite.RunCLI([]string{"images"})
}

func init() {
	allSuites = append(allSuites, new(ImagesSuite))
}
