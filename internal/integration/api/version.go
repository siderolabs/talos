// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"

	"github.com/talos-systems/talos/internal/integration/base"
)

// VersionSuite verifies version API
type VersionSuite struct {
	base.APISuite
}

// SuiteName ...
func (suite *VersionSuite) SuiteName() string {
	return "api.VersionSuite"
}

// SetupSuite ...
func (suite *VersionSuite) SetupSuite() {
	suite.InitClient()
}

// TearDownSuite ...
func (suite *VersionSuite) TearDownSuite() {
	if suite.Client != nil {
		suite.Assert().NoError(suite.Client.Close())
	}
}

// TestExpectedVersionMaster verifies master node version matches expected
func (suite *VersionSuite) TestExpectedVersionMaster() {
	v, err := suite.Client.Version(context.Background())
	suite.Require().NoError(err)

	suite.Assert().Equal(suite.Version, v.Response[0].Version.Tag)
}

func init() {
	allSuites = append(allSuites, new(VersionSuite))
}
