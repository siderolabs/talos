// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api
// +build integration_api

package api

import (
	"context"
	"io"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// EtcdSuite ...
type EtcdSuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EtcdSuite) SuiteName() string {
	return "api.EtcdSuite"
}

// SetupTest ...
func (suite *EtcdSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	// make sure we abort at some point in time, but give enough room for Etcds
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *EtcdSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestEtcdForfeitLeadership tests moving etcd leadership to another member.
func (suite *EtcdSuite) TestEtcdForfeitLeadership() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state etcd test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodes(suite.ctx).NodesByType(machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	var leader string

	for _, node := range nodes {
		resp, err := suite.Client.EtcdForfeitLeadership(client.WithNodes(suite.ctx, node), &machineapi.EtcdForfeitLeadershipRequest{})
		suite.Require().NoError(err)

		if resp.Messages[0].GetMember() != "" {
			leader = resp.Messages[0].GetMember()

			suite.T().Log("Moved leadership to", leader)
		}
	}

	suite.Assert().NotEmpty(leader)
}

// TestEtcdLeaveCluster tests removing an etcd member.
func (suite *EtcdSuite) TestEtcdLeaveCluster() {
	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodes(suite.ctx).NodesByType(machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	node := nodes[len(nodes)-1]

	suite.T().Log("Removing etcd member", node)

	nodeCtx := client.WithNodes(suite.ctx, node)

	_, err := suite.Client.EtcdForfeitLeadership(nodeCtx, &machineapi.EtcdForfeitLeadershipRequest{})
	suite.Require().NoError(err)

	err = suite.Client.EtcdLeaveCluster(nodeCtx, &machineapi.EtcdLeaveClusterRequest{})
	suite.Require().NoError(err)

	services, err := suite.Client.ServiceInfo(nodeCtx, "etcd")
	suite.Require().NoError(err)

	for _, service := range services {
		if service.Service.Id == "etcd" {
			suite.Assert().Equal("Finished", service.Service.State)
		}
	}

	stream, err := suite.Client.MachineClient.List(nodeCtx, &machineapi.ListRequest{Root: constants.EtcdDataPath})
	suite.Require().NoError(err)

	for {
		var info *machineapi.FileInfo

		info, err = stream.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				break
			}
		}

		suite.Assert().Equal("rpc error: code = Unknown desc = lstat /var/lib/etcd: no such file or directory", info.Metadata.Error)
	}

	// NB: Reboot the node so that it can rejoin the etcd cluster. This allows us
	// to check the cluster health and catch any issues in rejoining.
	suite.AssertRebooted(suite.ctx, node, func(nodeCtx context.Context) error {
		_, err = suite.Client.MachineClient.Reboot(nodeCtx, &machineapi.RebootRequest{})

		return err
	}, 10*time.Minute)
}

func init() {
	allSuites = append(allSuites, new(EtcdSuite))
}
