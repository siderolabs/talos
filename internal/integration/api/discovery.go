// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

// DiscoverySuite verifies Discovery API.
type DiscoverySuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
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
	node := suite.RandomDiscoveredNodeInternalIP()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)
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
	nodes := suite.DiscoverNodes(suite.ctx).Nodes()

	expectedTalosVersion := fmt.Sprintf("Talos (%s)", suite.Version)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node.InternalIP.String())

		members := suite.getMembers(nodeCtx)

		suite.Assert().Len(members, len(nodes))

		// do basic check against discovered nodes
		for _, expectedNode := range nodes {
			nodeAddresses := xslices.Map(expectedNode.IPs, func(t netip.Addr) string {
				return t.String()
			})

			found := false

			for _, member := range members {
				memberAddresses := xslices.Map(member.TypedSpec().Addresses, func(t netip.Addr) string {
					return t.String()
				})

				if maps.Contains(xslices.ToSet(memberAddresses), nodeAddresses) {
					found = true

					break
				}

				if found {
					break
				}
			}

			suite.Assert().True(found, "addr %q", nodeAddresses)
		}

		// if cluster information is available, perform additional checks
		if suite.Cluster == nil {
			continue
		}

		memberByName := xslices.ToMap(members,
			func(member *cluster.Member) (string, *cluster.Member) {
				return member.Metadata().ID(), member
			},
		)

		memberByIP := make(map[netip.Addr]*cluster.Member)

		for _, member := range members {
			for _, addr := range member.TypedSpec().Addresses {
				memberByIP[addr] = member
			}
		}

		nodesInfo := suite.Cluster.Info().Nodes

		for _, nodeInfo := range nodesInfo {
			matchingMember := memberByName[nodeInfo.Name]

			var matchingMemberByIP *cluster.Member

			for _, nodeIP := range nodeInfo.IPs {
				matchingMemberByIP = memberByIP[nodeIP]

				break
			}

			// if hostnames are not set via DHCP, use match by IP
			if matchingMember == nil {
				matchingMember = matchingMemberByIP
			}

			suite.Require().NotNil(matchingMember)

			suite.Assert().Equal(nodeInfo.Type, matchingMember.TypedSpec().MachineType)
			suite.Assert().Equal(expectedTalosVersion, matchingMember.TypedSpec().OperatingSystem)

			for _, nodeIP := range nodeInfo.IPs {
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
	node := suite.RandomDiscoveredNodeInternalIP()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	var registries []string

	if provider.Cluster().Discovery().Registries().Kubernetes().Enabled() {
		registries = append(registries, "k8s/")
	}

	if provider.Cluster().Discovery().Registries().Service().Enabled() {
		registries = append(registries, "service/")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		members := suite.getMembers(nodeCtx)
		localIdentity := suite.getNodeIdentity(nodeCtx)

		// raw affiliates don't contain the local node
		expectedRawAffiliates := len(registries) * (len(members) - 1)

		var rawAffiliates []*cluster.Affiliate

		for range 30 {
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

				stripDomain := func(s string) string { return strings.Split(s, ".")[0] }

				// registries can be a bit inconsistent, e.g. whether they report fqdn or just hostname
				suite.Assert().Contains([]string{member.TypedSpec().Hostname, stripDomain(member.TypedSpec().Hostname)}, rawAffiliate.TypedSpec().Hostname)

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
	node := suite.RandomDiscoveredNodeInternalIP()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if !provider.Machine().Network().KubeSpan().Enabled() {
		suite.T().Skip("KubeSpan is disabled")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		rtestutils.AssertLength[*kubespan.PeerSpec](nodeCtx, suite.T(), suite.Client.COSI, len(nodes)-1)
		rtestutils.AssertLength[*kubespan.PeerStatus](nodeCtx, suite.T(), suite.Client.COSI, len(nodes)-1)

		rtestutils.AssertAll[*kubespan.PeerStatus](nodeCtx, suite.T(), suite.Client.COSI,
			func(status *kubespan.PeerStatus, asrt *assert.Assertions) {
				asrt.Equal(kubespan.PeerStateUp, status.TypedSpec().State)
				asrt.False(value.IsZero(status.TypedSpec().Endpoint))
				asrt.Greater(status.TypedSpec().ReceiveBytes, int64(0))
				asrt.Greater(status.TypedSpec().TransmitBytes, int64(0))
			})
	}
}

// TestKubeSpanExtraEndpoints verifies that KubeSpan peer specs are updated with extra endpoints.
func (suite *DiscoverySuite) TestKubeSpanExtraEndpoints() {
	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("not running Talos kernel")
	}

	// check that cluster has KubeSpan enabled
	node := suite.RandomDiscoveredNodeInternalIP()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if !provider.Machine().Network().KubeSpan().Enabled() {
		suite.T().Skip("KubeSpan is disabled")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	if len(nodes) < 2 {
		suite.T().Skip("need at least two nodes for this test")
	}

	perm := rand.Perm(len(nodes))

	checkNode := nodes[perm[0]]
	targetNode := nodes[perm[1]]

	mockEndpoint := netip.MustParseAddrPort("169.254.121.121:5820")

	// inject extra endpoint to target node
	cfgDocument := network.NewKubespanEndpointsV1Alpha1()
	cfgDocument.ExtraAnnouncedEndpointsConfig = []netip.AddrPort{mockEndpoint}

	suite.T().Logf("injecting extra endpoint %s to node %s", mockEndpoint, targetNode)
	suite.PatchMachineConfig(client.WithNode(suite.ctx, targetNode), cfgDocument)

	targetIdentity, err := safe.ReaderGetByID[*kubespan.Identity](client.WithNode(suite.ctx, targetNode), suite.Client.COSI, kubespan.LocalIdentity)
	suite.Require().NoError(err)

	suite.T().Logf("checking extra endpoint %s on node %s", mockEndpoint, checkNode)
	rtestutils.AssertResources(client.WithNode(suite.ctx, checkNode), suite.T(), suite.Client.COSI, []string{targetIdentity.TypedSpec().PublicKey},
		func(peer *kubespan.PeerSpec, asrt *assert.Assertions) {
			asrt.Contains(peer.TypedSpec().Endpoints, mockEndpoint)
		},
	)

	// the extra endpoints disappears with a timeout from the discovery service, so can't assert on that
	suite.T().Logf("removin extra endpoint %s from node %s", mockEndpoint, targetNode)
	suite.RemoveMachineConfigDocuments(client.WithNode(suite.ctx, targetNode), cfgDocument.MetaKind)
}

func (suite *DiscoverySuite) getMembers(nodeCtx context.Context) []*cluster.Member {
	items, err := safe.StateListAll[*cluster.Member](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	return safe.ToSlice(items, func(m *cluster.Member) *cluster.Member { return m })
}

func (suite *DiscoverySuite) getNodeIdentity(nodeCtx context.Context) *cluster.Identity {
	identity, err := safe.StateGet[*cluster.Identity](nodeCtx, suite.Client.COSI, resource.NewMetadata(cluster.NamespaceName, cluster.IdentityType, cluster.LocalIdentity, resource.VersionUndefined))
	suite.Require().NoError(err)

	return identity
}

func (suite *DiscoverySuite) getAffiliates(nodeCtx context.Context, namespace resource.Namespace) []*cluster.Affiliate {
	var result []*cluster.Affiliate

	items, err := safe.StateList[*cluster.Affiliate](nodeCtx, suite.Client.COSI, resource.NewMetadata(namespace, cluster.AffiliateType, "", resource.VersionUndefined))
	suite.Require().NoError(err)

	items.ForEach(func(item *cluster.Affiliate) { result = append(result, item) })

	return result
}

func init() {
	allSuites = append(allSuites, new(DiscoverySuite))
}
