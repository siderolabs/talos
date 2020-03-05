// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/retry"
)

type RebootSuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *RebootSuite) SuiteName() string {
	return "api.RebootSuite"
}

// SetupTest ...
func (suite *RebootSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for reboots
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *RebootSuite) TearDownTest() {
	suite.ctxCancel()
}

// TestRebootNodeByNode reboots cluster node by node, waiting for health between reboots.
func (suite *RebootSuite) TestRebootNodeByNode() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodes()
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		suite.T().Log("rebooting node", node)

		func(node string) {
			// timeout for single node reboot
			ctx, ctxCancel := context.WithTimeout(suite.ctx, 5*time.Minute)
			defer ctxCancel()

			nodeCtx := client.WithNodes(ctx, node)

			// read uptime before reboot
			uptimeBefore, err := suite.ReadUptime(nodeCtx)
			suite.Require().NoError(err)

			suite.Assert().NoError(suite.Client.Reboot(nodeCtx))

			var uptimeAfter float64

			suite.Require().NoError(retry.Constant(3 * time.Minute).Retry(func() error {
				uptimeAfter, err = suite.ReadUptime(nodeCtx)
				if err != nil {
					// API might be unresponsive during reboot
					return retry.ExpectedError(err)
				}

				if uptimeAfter >= uptimeBefore {
					// uptime should go down after reboot
					return retry.ExpectedError(fmt.Errorf("uptime didn't go down: before %f, after %f", uptimeBefore, uptimeAfter))
				}

				return nil
			}))

			if suite.Cluster != nil {
				// without cluster state we can't do deep checks, but basic reboot test still works
				// NB: using `ctx` here to have client talking to init node by default
				suite.AssertClusterHealthy(ctx)
			}
		}(node)

	}
}

// TestRebootAllNodes reboots all cluster nodes at the same time.
func (suite *RebootSuite) TestRebootAllNodes() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodes()
	suite.Require().NotEmpty(nodes)

	errCh := make(chan error, len(nodes))

	var initialUptime sync.Map

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				nodeCtx := client.WithNodes(suite.ctx, node)

				// read uptime before reboot
				uptimeBefore, err := suite.ReadUptime(nodeCtx)
				if err != nil {
					return fmt.Errorf("error reading initial uptime (node %q): %w", node, err)
				}

				initialUptime.Store(node, uptimeBefore)
				return nil
			}()
		}(node)
	}

	for range nodes {
		suite.Require().NoError(<-errCh)
	}

	allNodesCtx := client.WithNodes(suite.ctx, nodes...)

	suite.Require().NoError(suite.Client.Reboot(allNodesCtx))

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				uptimeBeforeInterface, ok := initialUptime.Load(node)
				if !ok {
					return fmt.Errorf("uptime record not found for %q", node)
				}

				uptimeBefore := uptimeBeforeInterface.(float64) //nolint: errcheck

				nodeCtx := client.WithNodes(suite.ctx, node)

				return retry.Constant(3 * time.Minute).Retry(func() error {
					uptimeAfter, err := suite.ReadUptime(nodeCtx)
					if err != nil {
						// API might be unresponsive during reboot
						return retry.ExpectedError(err)
					}

					if uptimeAfter >= uptimeBefore {
						// uptime should go down after reboot
						return retry.ExpectedError(fmt.Errorf("uptime didn't go down: before %f, after %f", uptimeBefore, uptimeAfter))
					}

					return nil
				})
			}()
		}(node)
	}

	for range nodes {
		suite.Assert().NoError(<-errCh)
	}

	if suite.Cluster != nil {
		// without cluster state we can't do deep checks, but basic reboot test still works
		// NB: using `ctx` here to have client talking to init node by default
		suite.AssertClusterHealthy(suite.ctx)
	}
}

func init() {
	allSuites = append(allSuites, new(RebootSuite))
}
