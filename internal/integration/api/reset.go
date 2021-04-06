// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// ResetSuite ...
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
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	// make sure we abort at some point in time, but give enough room for Resets
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *ResetSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestResetNodeByNode Resets cluster node by node, waiting for health between Resets.
func (suite *ResetSuite) TestResetNodeByNode() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	initNodeAddress := ""

	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == machine.TypeInit {
			initNodeAddress = node.IPs[0].String()

			break
		}
	}

	nodes := suite.DiscoverNodes().Nodes()
	suite.Require().NotEmpty(nodes)

	sort.Strings(nodes)

	for _, node := range nodes {
		if node == initNodeAddress {
			// due to the bug with etcd cluster build for the init node after Reset(), skip resetting first node
			// there's no problem if bootstrap API was used, so this check only protects legacy init nodes
			suite.T().Log("Skipping init node", node, "due to known issue with etcd")

			continue
		}

		suite.T().Log("Resetting node", node)

		preReset, err := suite.HashKubeletCert(suite.ctx, node)
		suite.Require().NoError(err)

		suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
			// force reboot after reset, as this is the only mode we can test
			return base.IgnoreGRPCUnavailable(suite.Client.Reset(nodeCtx, true, true))
		}, 10*time.Minute)

		suite.ClearConnectionRefused(suite.ctx, node)

		postReset, err := suite.HashKubeletCert(suite.ctx, node)
		suite.Require().NoError(err)

		suite.Assert().NotEqual(preReset, postReset, "reset should lead to new kubelet cert being generated")
	}
}

// TestResetNoGraceful resets a worker in !graceful to test the flow.
//
// We can't reset control plane node in !graceful mode as it won't be able to join back the cluster.
func (suite *ResetSuite) TestResetNoGraceful() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	node := suite.RandomDiscoveredNode(machine.TypeJoin)

	suite.T().Log("Resetting node !graceful", node)

	preReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
		// force reboot after reset, as this is the only mode we can test
		return base.IgnoreGRPCUnavailable(suite.Client.Reset(nodeCtx, false, true))
	}, 5*time.Minute)

	suite.ClearConnectionRefused(suite.ctx, node)

	postReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Assert().NotEqual(preReset, postReset, "reset should lead to new kubelet cert being generated")
}

// TestResetWithSpecEphemeral resets only ephemeral partition on the node.
//
//nolint:dupl
func (suite *ResetSuite) TestResetWithSpecEphemeral() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	node := suite.RandomDiscoveredNode()

	suite.T().Log("Resetting node with spec=[EPHEMERAL]", node)

	preReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
		// force reboot after reset, as this is the only mode we can test
		return base.IgnoreGRPCUnavailable(suite.Client.ResetGeneric(nodeCtx, &machineapi.ResetRequest{
			Reboot:   true,
			Graceful: true,
			SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
				{
					Label: constants.EphemeralPartitionLabel,
					Wipe:  true,
				},
			},
		}))
	}, 5*time.Minute)

	suite.ClearConnectionRefused(suite.ctx, node)

	postReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Assert().NotEqual(preReset, postReset, "reset should lead to new kubelet cert being generated")
}

// TestResetWithSpecState resets only state partition on the node.
//
// As ephemeral partition is not reset, so kubelet cert shouldn't change.
//
//nolint:dupl
func (suite *ResetSuite) TestResetWithSpecState() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	node := suite.RandomDiscoveredNode()

	suite.T().Log("Resetting node with spec=[STATE]", node)

	preReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
		// force reboot after reset, as this is the only mode we can test
		return base.IgnoreGRPCUnavailable(suite.Client.ResetGeneric(nodeCtx, &machineapi.ResetRequest{
			Reboot:   true,
			Graceful: true,
			SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
				{
					Label: constants.StatePartitionLabel,
					Wipe:  true,
				},
			},
		}))
	}, 5*time.Minute)

	suite.ClearConnectionRefused(suite.ctx, node)

	postReset, err := suite.HashKubeletCert(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Assert().Equal(preReset, postReset, "ephemeral partition was not reset")
}

func init() {
	allSuites = append(allSuites, new(ResetSuite))
}
