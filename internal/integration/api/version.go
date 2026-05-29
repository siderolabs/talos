// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
	configmachine "github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// VersionSuite verifies version API.
type VersionSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *VersionSuite) SuiteName() string {
	return "api.VersionSuite"
}

// SetupTest ...
func (suite *VersionSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *VersionSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestExpectedVersionMaster verifies master node version matches expected.
func (suite *VersionSuite) TestExpectedVersionMaster() {
	v, err := suite.Client.Version(suite.ctx)
	suite.Require().NoError(err)

	node := suite.RandomDiscoveredNodeInternalIP(configmachine.TypeControlPlane)

	clientCtx := client.WithNode(suite.ctx, node)
	version, err := safe.StateGetByID[*runtime.Version](clientCtx, suite.Client.COSI, "version")
	suite.Require().NoError(err)
	suite.Require().NotNil(version)

	suite.Assert().Equal(v.Messages[0].Version.Tag, version.TypedSpec().Version)

	suite.Assert().Equal(suite.Version, v.Messages[0].Version.Tag)
	suite.Assert().Equal(suite.GoVersion, v.Messages[0].Version.GoVersion)
}

// TestSameVersionCluster verifies that all the nodes are on the same version.
func (suite *VersionSuite) TestSameVersionCluster() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		respCh := multiplex.Unary(suite.ctx, nodes, func(ctx context.Context) (*machine.VersionResponse, error) {
			return suite.Client.Version(ctx)
		})

		var firstVersion string

		for resp := range respCh {
			if !asrt.NoError(resp.Err) {
				continue
			}

			if !asrt.NotEmpty(resp.Payload.Messages) {
				continue
			}

			if firstVersion == "" {
				firstVersion = resp.Payload.Messages[0].Version.Tag
			} else {
				asrt.Equal(firstVersion, resp.Payload.Messages[0].Version.Tag)
			}
		}
	}, time.Minute, time.Second)
}

func init() {
	allSuites = append(allSuites, new(VersionSuite))
}
