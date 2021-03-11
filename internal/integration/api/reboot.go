// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/client"
)

// RebootSuite ...
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
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	// make sure we abort at some point in time, but give enough room for reboots
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *RebootSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestRebootNodeByNode reboots cluster node by node, waiting for health between reboots.
func (suite *RebootSuite) TestRebootNodeByNode() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodes().Nodes()
	suite.Require().NotEmpty(nodes)

	for _, node := range nodes {
		suite.T().Log("rebooting node", node)

		suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 10*time.Minute)
	}
}

// TestRebootAllNodes reboots all cluster nodes at the same time.
//
//nolint:gocyclo
func (suite *RebootSuite) TestRebootAllNodes() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	nodes := suite.DiscoverNodes().Nodes()
	suite.Require().NotEmpty(nodes)

	errCh := make(chan error, len(nodes))

	var initialBootID sync.Map

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				nodeCtx := client.WithNodes(suite.ctx, node)

				// read boot_id before reboot
				bootIDBefore, err := suite.ReadBootID(nodeCtx)
				if err != nil {
					return fmt.Errorf("error reading initial bootID (node %q): %w", node, err)
				}

				initialBootID.Store(node, bootIDBefore)

				return nil
			}()
		}(node)
	}

	for range nodes {
		suite.Require().NoError(<-errCh)
	}

	allNodesCtx := client.WithNodes(suite.ctx, nodes...)

	err := base.IgnoreGRPCUnavailable(suite.Client.Reboot(allNodesCtx))

	suite.Require().NoError(err)

	for _, node := range nodes {
		go func(node string) {
			errCh <- func() error {
				bootIDBeforeInterface, ok := initialBootID.Load(node)
				if !ok {
					return fmt.Errorf("bootID record not found for %q", node)
				}

				bootIDBefore := bootIDBeforeInterface.(string) //nolint:errcheck,forcetypeassert

				nodeCtx := client.WithNodes(suite.ctx, node)

				return retry.Constant(10 * time.Minute).Retry(func() error {
					requestCtx, requestCtxCancel := context.WithTimeout(nodeCtx, 5*time.Second)
					defer requestCtxCancel()

					bootIDAfter, err := suite.ReadBootID(requestCtx)
					if err != nil {
						// API might be unresponsive during reboot
						return retry.ExpectedError(fmt.Errorf("error reading bootID for node %q: %w", node, err))
					}

					if bootIDAfter == bootIDBefore {
						// bootID should be different after reboot
						return retry.ExpectedError(fmt.Errorf("bootID didn't change for node %q: before %s, after %s", node, bootIDBefore, bootIDAfter))
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
