// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
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

func (suite *RebootSuite) readUptime(ctx context.Context) (float64, error) {
	suite.T().Skip("test disabled due to issues with single endpoint")

	reader, errCh, err := suite.Client.Read(ctx, "/proc/uptime")
	if err != nil {
		return 0, err
	}

	defer reader.Close() //nolint: errcheck

	var uptime float64

	n, err := fmt.Fscanf(reader, "%f", &uptime)
	if err != nil {
		return 0, err
	}

	if n != 1 {
		return 0, fmt.Errorf("not all fields scanned: %d", n)
	}

	_, err = io.Copy(ioutil.Discard, reader)
	if err != nil {
		return 0, err
	}

	for err = range errCh {
		if err != nil {
			return 0, err
		}
	}

	return uptime, reader.Close()
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
			uptimeBefore, err := suite.readUptime(nodeCtx)
			suite.Require().NoError(err)

			suite.Assert().NoError(suite.Client.Reboot(nodeCtx))

			var uptimeAfter float64

			suite.Require().NoError(retry.Constant(3 * time.Minute).Retry(func() error {
				uptimeAfter, err = suite.readUptime(nodeCtx)
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

func init() {
	allSuites = append(allSuites, new(RebootSuite))
}
