// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/retry"
)

type ResetSuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ResetSuite) SuiteName() string {
	return "api.ResetSuite"
}

// SetupTest ...
func (suite *ResetSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for Resets
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *ResetSuite) TearDownTest() {
	suite.ctxCancel()
}

// TestResetNodeByNode Resets cluster node by node, waiting for health between Resets.
func (suite *ResetSuite) TestResetNodeByNode() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodes()
	suite.Require().NotEmpty(nodes)

	sort.Strings(nodes)

	for i, node := range nodes {
		if i == 0 {
			// first node should be init node, due to bug with etcd cluster build for init node
			// and Reset(), skip resetting first node
			suite.T().Log("Skipping init node", node, "due to known issue with etcd")
			continue
		}

		suite.T().Log("Resetting node", node)

		func(node string) {
			// timeout for single node Reset
			ctx, ctxCancel := context.WithTimeout(suite.ctx, 5*time.Minute)
			defer ctxCancel()

			nodeCtx := client.WithNodes(ctx, node)

			// read uptime before Reset
			uptimeBefore, err := suite.ReadUptime(nodeCtx)
			suite.Require().NoError(err)

			// force reboot after reset, as this is the only mode we can test
			suite.Assert().NoError(suite.Client.Reset(nodeCtx, true, true))

			var uptimeAfter float64

			suite.Require().NoError(retry.Constant(3 * time.Minute).Retry(func() error {
				uptimeAfter, err = suite.ReadUptime(nodeCtx)
				if err != nil {
					// API might be unresponsive during reboot
					return retry.ExpectedError(err)
				}

				if uptimeAfter >= uptimeBefore {
					// uptime should go down after Reset, as it reboots the node
					return retry.ExpectedError(fmt.Errorf("uptime didn't go down: before %f, after %f", uptimeBefore, uptimeAfter))
				}

				return nil
			}))

			// TODO: there is no good way to assert that node was reset and disk contents were really wiped

			// NB: using `ctx` here to have client talking to init node by default
			suite.AssertClusterHealthy(ctx)
		}(node)

	}
}

func init() {
	allSuites = append(allSuites, new(ResetSuite))
}
