// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	cfg "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/role"
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
			cli, err := client.New(
				suite.ctx,
				client.WithConfig(suite.Talosconfig),
				client.WithEndpoints(endpoint),
			)
			suite.Require().NoError(err)

			defer cli.Close() //nolint:errcheck

			// try with multiple nodes
			resp, err := cli.Version(client.WithNodes(suite.ctx, nodes...)) //nolint:staticcheck // testing deprecated method for backward compatibility
			suite.Require().NoError(err)
			suite.Assert().Len(resp.Messages, len(nodes))

			// try with 'nodes' but a single node at a time
			for _, node := range nodes {
				resp, err = cli.Version(client.WithNodes(suite.ctx, node)) //nolint:staticcheck // testing deprecated method for backward compatibility
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
			cli, err := client.New(
				suite.ctx,
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
				_, err = cli.Version(client.WithNodes(suite.ctx, node)) //nolint:staticcheck // testing deprecated method for backward compatibility
				suite.Require().Error(err)
				suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err))

				// 'node'
				_, err = cli.Version(client.WithNode(suite.ctx, node))
				suite.Require().Error(err)
				suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err))
			}

			// try with 'nodes' but a single node (node itself)
			resp, err := cli.Version(client.WithNodes(suite.ctx, endpoint)) //nolint:staticcheck // testing deprecated method for backward compatibility
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

	suite.T().Logf(
		"original config size: %d (%s), target size is %d (%s)",
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

// TestPKIMismatch verifies that PKI mismatch is handled correctly.
func (suite *ApidSuite) TestPKIMismatch() {
	bundle, err := secrets.NewBundle(secrets.NewClock(), cfg.TalosVersionCurrent)
	suite.Require().NoError(err)

	cert, err := bundle.GenerateTalosAPIClientCertificate(role.MakeSet(role.Admin))
	suite.Require().NoError(err)

	suite.Require().Contains(suite.Talosconfig.Contexts, suite.Talosconfig.Context)

	caCrt, err := base64.StdEncoding.DecodeString(suite.Talosconfig.Contexts[suite.Talosconfig.Context].CA)
	suite.Require().NoError(err)

	wrongConfig := clientconfig.NewConfig("wrong", suite.Talosconfig.Contexts[suite.Talosconfig.Context].Endpoints, caCrt, cert)

	wrongClient, err := client.New(suite.ctx, client.WithConfig(wrongConfig))
	suite.Require().NoError(err)

	_, err = wrongClient.Version(suite.ctx)
	suite.Require().Error(err)
	suite.Assert().Equal(codes.Unavailable, client.StatusCode(err))
	suite.Assert().True(
		strings.Contains(err.Error(), "remote error: tls: unknown certificate authority") ||
			strings.Contains(err.Error(), "write: broken pipe") ||
			strings.Contains(err.Error(), "write: connection reset by peer"),
		"unexpected error: %v", err,
	)

	suite.Require().NoError(wrongClient.Close())
}

// TestImpersonationWithoutRole verifies that the impersonation header is rejected when the client
// doesn't have os:impersonator role, whatever roles the client has otherwise.
func (suite *ApidSuite) TestImpersonationWithoutRole() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	cpNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	for _, tt := range []struct {
		name  string
		roles []role.Role
	}{
		{
			name:  "reader",
			roles: []role.Role{role.Reader},
		},
		{
			name:  "admin",
			roles: []role.Role{role.Admin},
		},
		{
			name:  "operator and reader",
			roles: []role.Role{role.Operator, role.Reader},
		},
	} {
		suite.Run(tt.name, func() {
			cli := suite.generateClient(tt.roles...)

			for _, node := range nodes {
				nodeCtx := client.WithNode(suite.ctx, node)

				// sanity check: the client works without the impersonation header
				_, err := cli.Version(nodeCtx)
				suite.Require().NoError(err)

				// any impersonation header is rejected, whether it escalates, downgrades or keeps the roles
				for _, impersonated := range []role.Role{role.Admin, role.Reader, role.Impersonator, tt.roles[0]} {
					_, err = cli.Version(withImpersonation(nodeCtx, impersonated))
					suite.Require().Error(err)
					suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)
					suite.Assert().ErrorContains(err, "impersonator role")
				}
			}

			// the header doesn't escalate access to admin-only APIs either
			_, err := cli.GenerateClientConfiguration(withImpersonation(client.WithNode(suite.ctx, cpNode), role.Admin), &machineapi.GenerateClientConfigurationRequest{
				Roles:  []string{string(role.Reader)},
				CrtTtl: durationpb.New(time.Hour),
			})
			suite.Require().Error(err)
			suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)
			suite.Assert().ErrorContains(err, "impersonator role")
		})
	}
}

// TestImpersonation verifies that a client with os:impersonator role can impersonate any role via the impersonation header,
// and that the impersonated roles are what gets authorized, including when the request is proxied between apid instances.
func (suite *ApidSuite) TestImpersonation() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	cpCtx := client.WithNode(suite.ctx, suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane))

	adminOnlyRequest := &machineapi.GenerateClientConfigurationRequest{
		Roles:  []string{string(role.Reader)},
		CrtTtl: durationpb.New(time.Hour),
	}

	suite.Run("impersonator only", func() {
		cli := suite.generateClient(role.Impersonator)

		for _, node := range nodes {
			nodeCtx := client.WithNode(suite.ctx, node)

			// os:impersonator alone doesn't grant access to anything
			_, err := cli.Version(nodeCtx)
			suite.Require().Error(err)
			suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)

			// impersonating a reader grants read-only access
			_, err = cli.Version(withImpersonation(nodeCtx, role.Reader))
			suite.Require().NoError(err)

			// impersonating an admin grants access as well
			_, err = cli.Version(withImpersonation(nodeCtx, role.Admin))
			suite.Require().NoError(err)

			// impersonating an unknown role grants nothing
			_, err = cli.Version(withImpersonation(nodeCtx, role.Role("os:nonexistent")))
			suite.Require().Error(err)
			suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)
		}

		// impersonated reader is denied admin-only APIs
		_, err := cli.GenerateClientConfiguration(withImpersonation(cpCtx, role.Reader), adminOnlyRequest)
		suite.Require().Error(err)
		suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)

		// impersonated admin is allowed admin-only APIs
		_, err = cli.GenerateClientConfiguration(withImpersonation(cpCtx, role.Admin), adminOnlyRequest)
		suite.Require().NoError(err)
	})

	suite.Run("impersonator and reader", func() {
		cli := suite.generateClient(role.Impersonator, role.Reader)

		for _, node := range nodes {
			nodeCtx := client.WithNode(suite.ctx, node)

			// without the header, the client's own roles apply
			_, err := cli.Version(nodeCtx)
			suite.Require().NoError(err)
		}

		// the client's own roles don't include admin
		_, err := cli.GenerateClientConfiguration(cpCtx, adminOnlyRequest)
		suite.Require().Error(err)
		suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)

		// the header replaces the client's own roles rather than being merged with them
		_, err = cli.GenerateClientConfiguration(withImpersonation(cpCtx, role.Admin), adminOnlyRequest)
		suite.Require().NoError(err)

		_, err = cli.GenerateClientConfiguration(withImpersonation(cpCtx, role.Operator), adminOnlyRequest)
		suite.Require().Error(err)
		suite.Assert().Equal(codes.PermissionDenied, client.StatusCode(err), "unexpected error: %v", err)
	})
}

// generateClient returns a Talos API client with a certificate carrying the given roles.
//
// The certificate is issued by the cluster via the GenerateClientConfiguration API, so it is trusted by apid.
func (suite *ApidSuite) generateClient(roles ...role.Role) *client.Client {
	cpNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	resp, err := suite.Client.GenerateClientConfiguration(client.WithNode(suite.ctx, cpNode), &machineapi.GenerateClientConfigurationRequest{
		Roles:  role.MakeSet(roles...).Strings(),
		CrtTtl: durationpb.New(time.Hour),
	})
	suite.Require().NoError(err)
	suite.Require().Len(resp.Messages, 1)

	config, err := clientconfig.FromBytes(resp.Messages[0].Talosconfig)
	suite.Require().NoError(err)

	config.Contexts[config.Context].Endpoints = suite.Client.GetEndpoints()

	cli, err := client.New(suite.ctx, client.WithConfig(config))
	suite.Require().NoError(err)

	suite.T().Cleanup(func() {
		suite.Assert().NoError(cli.Close())
	})

	return cli
}

// withImpersonation sets the impersonation header in the outgoing gRPC metadata.
func withImpersonation(ctx context.Context, roles ...role.Role) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()

	md.Set(constants.APIAuthzRoleMetadataKey, role.MakeSet(roles...).Strings()...)

	return metadata.NewOutgoingContext(ctx, md)
}

func init() {
	allSuites = append(allSuites, new(ApidSuite))
}
