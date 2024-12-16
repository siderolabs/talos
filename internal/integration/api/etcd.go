// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EtcdSuite ...
type EtcdSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EtcdSuite) SuiteName() string {
	return "api.EtcdSuite"
}

// SetupTest ...
func (suite *EtcdSuite) SetupTest() {
	// make sure we abort at some point in time, but give enough room for Etcds
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *EtcdSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestForfeitLeadership tests moving etcd leadership to another member.
func (suite *EtcdSuite) TestForfeitLeadership() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state etcd test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	var leader string

	for _, node := range nodes {
		resp, err := suite.Client.EtcdForfeitLeadership(
			client.WithNodes(suite.ctx, node),
			&machineapi.EtcdForfeitLeadershipRequest{},
		)
		suite.Require().NoError(err)

		if resp.Messages[0].GetMember() != "" {
			leader = resp.Messages[0].GetMember()

			suite.T().Log("Moved leadership to", leader)
		}
	}

	suite.Assert().NotEmpty(leader)
}

// TestLeaveCluster tests removing an etcd member.
//
//nolint:gocyclo
func (suite *EtcdSuite) TestLeaveCluster() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

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

		suite.Assert().Equal(
			"rpc error: code = Unknown desc = lstat /var/lib/etcd: no such file or directory",
			info.Metadata.Error,
		)
	}

	// NB: Reboot the node so that it can rejoin the etcd cluster. This allows us
	// to check the cluster health and catch any issues in rejoining.
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = suite.Client.MachineClient.Reboot(nodeCtx, &machineapi.RebootRequest{})

			return err
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)
}

// TestMembers verifies that etcd members as resources and API response are consistent.
func (suite *EtcdSuite) TestMembers() {
	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	// map member ID to hostname
	etcdMembers := map[string]string{}

	for _, node := range nodes {
		member, err := safe.StateGet[*etcd.Member](client.WithNode(suite.ctx, node), suite.Client.COSI, etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID).Metadata())
		suite.Require().NoError(err)

		hostname, err := safe.StateGet[*network.HostnameStatus](client.WithNode(suite.ctx, node), suite.Client.COSI, network.NewHostnameStatus(network.NamespaceName, network.HostnameID).Metadata())
		suite.Require().NoError(err)

		etcdMembers[member.TypedSpec().MemberID] = hostname.TypedSpec().Hostname
	}

	suite.Assert().Len(etcdMembers, len(nodes))

	resp, err := suite.Client.EtcdMemberList(suite.ctx, &machineapi.EtcdMemberListRequest{})
	suite.Require().NoError(err)

	count := 0

	for _, message := range resp.GetMessages() {
		for _, member := range message.GetMembers() {
			count++

			memberID := etcd.FormatMemberID(member.GetId())

			suite.Assert().Contains(etcdMembers, memberID)
			suite.Assert().Equal(etcdMembers[memberID], member.GetHostname())
		}
	}

	suite.Assert().Equal(len(etcdMembers), count)
}

// TestRemoveMember tests removing an etcd member forcefully.
func (suite *EtcdSuite) TestRemoveMember() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot (and reset)")
	}

	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reset test is not reliable (can't wait for cluster readiness in between resets)")
	}

	nodes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane)

	if len(nodes) < 3 {
		suite.T().Skip("test only can be run on HA etcd clusters")
	}

	controlNode, nodeToRemove := nodes[len(nodes)-1], nodes[0]

	suite.T().Log("Removing etcd member", nodeToRemove)

	removeCtx := client.WithNode(suite.ctx, nodeToRemove)
	controlCtx := client.WithNode(suite.ctx, controlNode)

	_, err := suite.Client.EtcdForfeitLeadership(removeCtx, &machineapi.EtcdForfeitLeadershipRequest{})
	suite.Require().NoError(err)

	member, err := safe.StateGet[*etcd.Member](removeCtx, suite.Client.COSI, etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID).Metadata())
	suite.Require().NoError(err)

	memberID, err := etcd.ParseMemberID(member.TypedSpec().MemberID)
	suite.Require().NoError(err)

	err = suite.Client.EtcdRemoveMemberByID(controlCtx, &machineapi.EtcdRemoveMemberByIDRequest{
		MemberId: memberID,
	})
	suite.Require().NoError(err)

	// verify that memberID disappeared from etcd member list
	resp, err := suite.Client.EtcdMemberList(controlCtx, &machineapi.EtcdMemberListRequest{})
	suite.Require().NoError(err)

	for _, message := range resp.GetMessages() {
		for _, member := range message.GetMembers() {
			suite.Assert().NotEqual(memberID, member.GetId())
		}
	}

	// NB: Reset the ephemeral the node so that it can rejoin the etcd cluster. This allows us
	// to check the cluster health and catch any issues in rejoining.
	suite.AssertRebooted(
		suite.ctx, nodeToRemove, func(nodeCtx context.Context) error {
			_, err = suite.Client.MachineClient.Reset(nodeCtx, &machineapi.ResetRequest{
				Reboot: true,
				SystemPartitionsToWipe: []*machineapi.ResetPartitionSpec{
					{
						Label: constants.EphemeralPartitionLabel,
						Wipe:  true,
					},
				},
			})

			return err
		}, 10*time.Minute,
		suite.CleanupFailedPods,
	)
}

func init() {
	allSuites = append(allSuites, new(EtcdSuite))
}
