// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Minute)
}

// TearDownTest ...
func (suite *ApidSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestControlPlaneRouting verify access to all nodes via each control plane node as an endpoints.
func (suite *ApidSuite) TestControlPlaneRouting() {
	if suite.Cluster == nil {
		suite.T().Skip("information about routable endpoints is not available")
	}

	if suite.APISuite.Endpoint != "" {
		suite.T().Skip("test skipped as custom endpoint is set")
	}

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
	if suite.Cluster == nil {
		suite.T().Skip("information about routable endpoints is not available")
	}

	if suite.APISuite.Endpoint != "" {
		suite.T().Skip("test skipped as custom endpoint is set")
	}

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

// TestBigPayload verifies that big payloads are handled correctly.
func (suite *ApidSuite) TestBigPayload() {
	if testing.Short() {
		suite.T().Skip("skipping test in short mode")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing big payload on node %s", node)

	// we are going to simulate a big payload by making machine configuration big enough
	cfg, err := safe.StateGetByID[*config.MachineConfig](nodeCtx, suite.Client.COSI, config.ActiveID)
	suite.Require().NoError(err)

	originalCfg, err := cfg.Container().Bytes()
	suite.Require().NoError(err)

	// the config is encoded twice in the resource gRPC message, so ensure that we can get to the one third of the size
	const targetConfigSize = constants.GRPCMaxMessageSize / 3

	suite.T().Logf("original config size: %d (%s), target size is %d (%s)",
		len(originalCfg), humanize.Bytes(uint64(len(originalCfg))), targetConfigSize, humanize.Bytes(uint64(targetConfigSize)),
	)

	bytesToAdd := targetConfigSize - len(originalCfg)
	if bytesToAdd <= 0 {
		suite.T().Skip("configuration is already big enough")
	}

	const commentLine = "# this is a comment line added to make the config bigger and bigger and bigger and bigger all the way\n"

	newConfig := slices.Concat(originalCfg, bytes.Repeat([]byte(commentLine), bytesToAdd/len(commentLine)+1))

	suite.Assert().Greater(len(newConfig), targetConfigSize)

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: newConfig,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})
	suite.Require().NoError(err)

	// now get the machine configuration back several times
	for range 5 {
		cfg, err = safe.StateGetByID[*config.MachineConfig](nodeCtx, suite.Client.COSI, config.ActiveID)
		suite.Require().NoError(err)

		// check that the configuration is the same
		newCfg, err := cfg.Container().Bytes()
		suite.Require().NoError(err)

		suite.Assert().Equal(newConfig, newCfg)
	}

	// revert the configuration
	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: originalCfg,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})
	suite.Require().NoError(err)
}

func init() {
	allSuites = append(allSuites, new(ApidSuite))
}
