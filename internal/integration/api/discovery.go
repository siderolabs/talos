// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api
// +build integration_api

package api

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"gopkg.in/yaml.v3"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubespan"
)

// DiscoverySuite verifies Discovery API.
type DiscoverySuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *DiscoverySuite) SuiteName() string {
	return "api.DiscoverySuite"
}

// SetupTest ...
func (suite *DiscoverySuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)

	// check that cluster has discovery enabled
	node := suite.RandomDiscoveredNode()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNodes(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if !provider.Cluster().Discovery().Enabled() {
		suite.T().Skip("cluster discovery is disabled")
	}
}

// TearDownTest ...
func (suite *DiscoverySuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestMembers checks that `talosctl get members` matches expected cluster discovery.
//
//nolint:gocyclo
func (suite *DiscoverySuite) TestMembers() {
	nodes := suite.DiscoverNodes(suite.ctx)
	expectedTalosVersion := fmt.Sprintf("Talos (%s)", suite.Version)

	for _, node := range nodes.Nodes() {
		nodeCtx := client.WithNodes(suite.ctx, node)

		members := suite.getMembers(nodeCtx)

		suite.Assert().Len(members, len(nodes.Nodes()))

		// do basic check against discovered nodes
		for _, expectedNode := range nodes.Nodes() {
			addr, err := netaddr.ParseIP(expectedNode)
			suite.Require().NoError(err)

			found := false

			for _, member := range members {
				for _, memberAddr := range member.TypedSpec().Addresses {
					if memberAddr.Compare(addr) == 0 {
						found = true

						break
					}
				}

				if found {
					break
				}
			}

			suite.Assert().True(found, "addr %s", addr)
		}

		// if cluster informantion is available, perform additional checks
		if suite.Cluster == nil {
			continue
		}

		memberByID := make(map[string]*cluster.Member)

		for _, member := range members {
			memberByID[member.Metadata().ID()] = member
		}

		nodesInfo := suite.Cluster.Info().Nodes

		for _, nodeInfo := range nodesInfo {
			matchingMember := memberByID[nodeInfo.Name]
			suite.Require().NotNil(matchingMember)

			suite.Assert().Equal(nodeInfo.Type, matchingMember.TypedSpec().MachineType)
			suite.Assert().Equal(expectedTalosVersion, matchingMember.TypedSpec().OperatingSystem)
			suite.Assert().Equal(nodeInfo.Name, matchingMember.TypedSpec().Hostname)

			for _, nodeIPStd := range nodeInfo.IPs {
				nodeIP, ok := netaddr.FromStdIP(nodeIPStd)
				suite.Assert().True(ok)

				found := false

				for _, memberAddr := range matchingMember.TypedSpec().Addresses {
					if memberAddr.Compare(nodeIP) == 0 {
						found = true

						break
					}
				}

				suite.Assert().True(found, "addr %s", nodeIP)
			}
		}
	}
}

// TestRegistries checks that all registries produce same raw Affiliate data.
func (suite *DiscoverySuite) TestRegistries() {
	registries := []string{"k8s/", "service/"}

	nodes := suite.DiscoverNodes(suite.ctx)

	for _, node := range nodes.Nodes() {
		nodeCtx := client.WithNodes(suite.ctx, node)

		members := suite.getMembers(nodeCtx)
		localIdentity := suite.getNodeIdentity(nodeCtx)

		// raw affiliates don't contain the local node
		expectedRawAffiliates := len(registries) * (len(members) - 1)

		var rawAffiliates []*cluster.Affiliate

		for i := 0; i < 30; i++ {
			rawAffiliates = suite.getAffiliates(nodeCtx, cluster.RawNamespaceName)

			if len(rawAffiliates) == expectedRawAffiliates {
				break
			}

			suite.T().Logf("waiting for cluster affiliates to be discovered: %d expected, %d found", expectedRawAffiliates, len(rawAffiliates))

			time.Sleep(2 * time.Second)
		}

		suite.Assert().Len(rawAffiliates, expectedRawAffiliates)

		rawAffiliatesByID := make(map[string]*cluster.Affiliate)

		for _, rawAffiliate := range rawAffiliates {
			rawAffiliatesByID[rawAffiliate.Metadata().ID()] = rawAffiliate
		}

		// every member except for local identity member should be discovered via each registry
		for _, member := range members {
			if member.TypedSpec().NodeID == localIdentity.TypedSpec().NodeID {
				continue
			}

			for _, registry := range registries {
				rawAffiliate := rawAffiliatesByID[registry+member.TypedSpec().NodeID]
				suite.Require().NotNil(rawAffiliate)

				suite.Assert().Equal(member.TypedSpec().Hostname, rawAffiliate.TypedSpec().Hostname)
				suite.Assert().Equal(member.TypedSpec().Addresses, rawAffiliate.TypedSpec().Addresses)
				suite.Assert().Equal(member.TypedSpec().OperatingSystem, rawAffiliate.TypedSpec().OperatingSystem)
				suite.Assert().Equal(member.TypedSpec().MachineType, rawAffiliate.TypedSpec().MachineType)
			}
		}
	}
}

// TestKubeSpanPeers verifies that KubeSpan peer specs are populated, and that peer statuses are available.
func (suite *DiscoverySuite) TestKubeSpanPeers() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("not running Talos kernel")
	}

	// check that cluster has KubeSpan enabled
	node := suite.RandomDiscoveredNode()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNodes(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if !provider.Machine().Network().KubeSpan().Enabled() {
		suite.T().Skip("KubeSpan is disabled")
	}

	nodes := suite.DiscoverNodes(suite.ctx).Nodes()

	for _, node := range nodes {
		nodeCtx := client.WithNodes(suite.ctx, node)

		peerSpecs := suite.getKubeSpanPeerSpecs(nodeCtx)
		suite.Assert().Len(peerSpecs, len(nodes)-1)

		peerStatuses := suite.getKubeSpanPeerStatuses(nodeCtx)
		suite.Assert().Len(peerStatuses, len(nodes)-1)

		for _, status := range peerStatuses {
			suite.Assert().Equal(kubespan.PeerStateUp, status.TypedSpec().State)
			suite.Assert().False(status.TypedSpec().Endpoint.IsZero())
			suite.Assert().Greater(status.TypedSpec().ReceiveBytes, int64(0))
			suite.Assert().Greater(status.TypedSpec().TransmitBytes, int64(0))
		}
	}
}

//nolint:dupl
func (suite *DiscoverySuite) getMembers(nodeCtx context.Context) []*cluster.Member {
	var result []*cluster.Member

	memberList, err := suite.Client.Resources.List(nodeCtx, cluster.NamespaceName, cluster.MemberType)
	suite.Require().NoError(err)

	for {
		resp, err := memberList.Recv()
		if err == io.EOF {
			break
		}

		suite.Require().NoError(err)

		if resp.Resource == nil {
			continue
		}

		// TODO: this is hackery to decode back into Member resource
		//       this should be fixed once we introduce protobuf properly
		b, err := yaml.Marshal(resp.Resource.Spec())
		suite.Require().NoError(err)

		member := cluster.NewMember(resp.Resource.Metadata().Namespace(), resp.Resource.Metadata().ID())

		suite.Require().NoError(yaml.Unmarshal(b, member.TypedSpec()))

		result = append(result, member)
	}

	return result
}

func (suite *DiscoverySuite) getNodeIdentity(nodeCtx context.Context) *cluster.Identity {
	list, err := suite.Client.Resources.Get(nodeCtx, cluster.NamespaceName, cluster.IdentityType, cluster.LocalIdentity)
	suite.Require().NoError(err)
	suite.Require().Len(list, 1)

	resp := list[0]

	// TODO: this is hackery to decode back into Member resource
	//       this should be fixed once we introduce protobuf properly
	b, err := yaml.Marshal(resp.Resource.Spec())
	suite.Require().NoError(err)

	identity := cluster.NewIdentity(resp.Resource.Metadata().Namespace(), resp.Resource.Metadata().ID())

	suite.Require().NoError(yaml.Unmarshal(b, identity.TypedSpec()))

	return identity
}

//nolint:dupl
func (suite *DiscoverySuite) getAffiliates(nodeCtx context.Context, namespace resource.Namespace) []*cluster.Affiliate {
	var result []*cluster.Affiliate

	affiliateList, err := suite.Client.Resources.List(nodeCtx, namespace, cluster.AffiliateType)
	suite.Require().NoError(err)

	for {
		resp, err := affiliateList.Recv()
		if err == io.EOF {
			break
		}

		suite.Require().NoError(err)

		if resp.Resource == nil {
			continue
		}

		// TODO: this is hackery to decode back into Affiliate resource
		//       this should be fixed once we introduce protobuf properly
		b, err := yaml.Marshal(resp.Resource.Spec())
		suite.Require().NoError(err)

		affiliate := cluster.NewAffiliate(resp.Resource.Metadata().Namespace(), resp.Resource.Metadata().ID())

		suite.Require().NoError(yaml.Unmarshal(b, affiliate.TypedSpec()))

		result = append(result, affiliate)
	}

	return result
}

//nolint:dupl
func (suite *DiscoverySuite) getKubeSpanPeerSpecs(nodeCtx context.Context) []*kubespan.PeerSpec {
	var result []*kubespan.PeerSpec

	peerList, err := suite.Client.Resources.List(nodeCtx, kubespan.NamespaceName, kubespan.PeerSpecType)
	suite.Require().NoError(err)

	for {
		resp, err := peerList.Recv()
		if err == io.EOF {
			break
		}

		suite.Require().NoError(err)

		if resp.Resource == nil {
			continue
		}

		// TODO: this is hackery to decode back into KubeSpanPeerSpec resource
		//       this should be fixed once we introduce protobuf properly
		b, err := yaml.Marshal(resp.Resource.Spec())
		suite.Require().NoError(err)

		peerSpec := kubespan.NewPeerSpec(resp.Resource.Metadata().Namespace(), resp.Resource.Metadata().ID())

		suite.Require().NoError(yaml.Unmarshal(b, peerSpec.TypedSpec()))

		result = append(result, peerSpec)
	}

	return result
}

//nolint:dupl
func (suite *DiscoverySuite) getKubeSpanPeerStatuses(nodeCtx context.Context) []*kubespan.PeerStatus {
	var result []*kubespan.PeerStatus

	peerList, err := suite.Client.Resources.List(nodeCtx, kubespan.NamespaceName, kubespan.PeerStatusType)
	suite.Require().NoError(err)

	for {
		resp, err := peerList.Recv()
		if err == io.EOF {
			break
		}

		suite.Require().NoError(err)

		if resp.Resource == nil {
			continue
		}

		// TODO: this is hackery to decode back into KubeSpanPeerStatus resource
		//       this should be fixed once we introduce protobuf properly
		b, err := yaml.Marshal(resp.Resource.Spec())
		suite.Require().NoError(err)

		peerStatus := kubespan.NewPeerStatus(resp.Resource.Metadata().Namespace(), resp.Resource.Metadata().ID())

		suite.Require().NoError(yaml.Unmarshal(b, peerStatus.TypedSpec()))

		result = append(result, peerStatus)
	}

	return result
}

func init() {
	allSuites = append(allSuites, new(DiscoverySuite))
}
