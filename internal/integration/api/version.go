// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
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

// TestExpectedVersionMaster verifies master node version matches expected
func (suite *VersionSuite) TestExpectedVersionMaster() {
	v, err := suite.Client.Version(context.Background())
	suite.Require().NoError(err)

	suite.Assert().Equal(suite.Version, v.Response[0].Version.Tag)
}

// TestSameVersionCluster verifies that all the nodes are on the same version
func (suite *VersionSuite) TestSameVersionCluster() {
	nodes := suite.DiscoverNodes()
	suite.Require().NotEmpty(nodes)

	ctx := client.WithTargets(context.Background(), nodes...)

	v, err := suite.Client.Version(ctx)
	suite.Require().NoError(err)

	suite.Require().Len(v.Response, len(nodes))

	expectedVersion := v.Response[0].Version.Tag
	for _, version := range v.Response {
		suite.Assert().Equal(expectedVersion, version.Version.Tag)
	}
}

func init() {
	allSuites = append(allSuites, new(VersionSuite))
}
