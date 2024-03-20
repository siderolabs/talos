// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ApidSuite verifies Discovery API.
type ApidSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ApidSuite) SuiteName() string {
	return "api.ApidSuite"
}

// SetupTest ...
func (suite *ApidSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)

	if suite.Cluster == nil {
		suite.T().Skip("information about routable endpoints is not available")
	}

	if suite.APISuite.Endpoint != "" {
		suite.T().Skip("test skipped as custom endpoint is set")
	}
}

// TearDownTest ...
func (suite *ApidSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestControlPlaneRouting verify access to all nodes via each control plane node as an endpoints.
func (suite *ApidSuite) TestControlPlaneRouting() {
	endpoints := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, endpoint := range endpoints {
		suite.Run(endpoint, func() {
			cli, err := client.New(suite.ctx,
				client.WithConfig(suite.Talosconfig),
				client.WithEndpoints(endpoint),
			)
			suite.Require().NoError(err)

			defer cli.Close() //nolint:errcheck

			// try with multiple nodes
			resp, err := cli.Version(client.WithNodes(suite.ctx, nodes...))
			suite.Require().NoError(err)
			suite.Assert().Len(resp.Messages, len(nodes))

			// try with 'nodes' but a single node at a time
			for _, node := range nodes {
				resp, err = cli.Version(client.WithNodes(suite.ctx, node))
				suite.Require().NoError(err)
				suite.Assert().Len(resp.Messages, 1)
			}

			// try with 'node'
			for _, node := range nodes {
				resp, err = cli.Version(client.WithNode(suite.ctx, node))
				suite.Require().NoError(err)
				suite.Assert().Len(resp.Messages, 1)
			}

			// try without any nodes set
			resp, err = cli.Version(suite.ctx)
			suite.Require().NoError(err)
			suite.Assert().Len(resp.Messages, 1)
		})
	}
}

// TestWorkerNoRouting verifies that worker nodes perform no routing.
func (suite *ApidSuite) TestWorkerNoRouting() {
	endpoints := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	if len(endpoints) == 0 {
		suite.T().Skip("no worker nodes found")
	}

	_, err := safe.StateGetByID[*network.NfTablesChain](client.WithNode(suite.ctx, endpoints[0]), suite.Client.COSI, "ingress")
	if err == nil {
		suite.T().Skip("worker nodes have ingress firewall enabled, skipping")
	}

	for _, endpoint := range endpoints {
		suite.Run(endpoint, func() {
			cli, err := client.New(suite.ctx,
				client.WithConfig(suite.Talosconfig),
				client.WithEndpoints(endpoint),
			)
			suite.Require().NoError(err)

			defer cli.Close() //nolint:errcheck

			// try every other node but the one we're connected to
			// there should be no routing
			for _, node := range nodes {
				if node == endpoint {
					continue
				}

				// 'nodes'
				_, err = cli.Version(client.WithNodes(suite.ctx, node))
				suite.Require().Error(err)
				suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err))

				// 'node'
				_, err = cli.Version(client.WithNode(suite.ctx, node))
				suite.Require().Error(err)
				suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err))
			}

			// try with 'nodes' but a single node (node itself)
			resp, err := cli.Version(client.WithNodes(suite.ctx, endpoint))
			suite.Require().NoError(err)
			suite.Assert().Len(resp.Messages, 1)

			// try with 'node' (node itself)
			resp, err = cli.Version(client.WithNode(suite.ctx, endpoint))
			suite.Require().NoError(err)
			suite.Assert().Len(resp.Messages, 1)

			// try without any nodes set
			resp, err = cli.Version(suite.ctx)
			suite.Require().NoError(err)
			suite.Assert().Len(resp.Messages, 1)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(ApidSuite))
}
