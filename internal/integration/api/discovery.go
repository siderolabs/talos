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
	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	clustertypes "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	resourcesconfig "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/version"
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

	if !discoveryServiceEnabled(provider) && !kubernetesRegistryEnabled(provider) {
		suite.T().Skip("cluster discovery is disabled")
	}
}

// discoveryServiceEnabled reports whether the discovery service registry is active,
// covering both the legacy cluster.discovery block and the new DiscoveryServiceConfig documents.
func discoveryServiceEnabled(provider config.Provider) bool {
	return len(provider.DiscoveryServiceConfigs()) > 0
}

// kubernetesRegistryEnabled reports whether the legacy Kubernetes discovery registry is active.
// The Kubernetes registry only exists in the legacy v1alpha1 config path.
func kubernetesRegistryEnabled(provider config.Provider) bool {
	return provider.Cluster().Discovery().Enabled() &&
		provider.Cluster().Discovery().Registries().Kubernetes().Enabled()
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

	expectedTalosVersion := fmt.Sprintf("%s (%s)", version.Name, suite.Version)

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

		memberByName := xslices.ToMap(
			members,
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

	if kubernetesRegistryEnabled(provider) {
		registries = append(registries, "k8s/")
	}

	if discoveryServiceEnabled(provider) {
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

// TestServiceEndpoints verifies that the cluster Config resource's ServiceEndpoints map reflects the
// configured discovery services, covering both the legacy cluster.discovery block (surfaced as a single
// "legacy" entry) and the new multi-doc DiscoveryServiceConfig documents.
func (suite *DiscoverySuite) TestServiceEndpoints() {
	node := suite.RandomDiscoveredNodeInternalIP()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if !discoveryServiceEnabled(provider) {
		suite.T().Skip("discovery service is disabled")
	}

	expected := xslices.Map(provider.DiscoveryServiceConfigs(), func(c configconfig.DiscoveryServiceConfig) cluster.ServiceEndpoint {
		addr, insecure, err := clustertypes.NormalizeEndpoint(c.Endpoint().String())
		suite.Require().NoError(err)

		return cluster.ServiceEndpoint{Name: c.Name(), Endpoint: addr, Insecure: insecure}
	})

	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{cluster.ConfigID},
		func(cfg *cluster.Config, asrt *assert.Assertions) {
			asrt.ElementsMatch(expected, cfg.TypedSpec().ServiceEndpoints)
		},
		rtestutils.WithNamespace(resourcesconfig.NamespaceName),
	)
}

// TestDiscoveryServiceConfigDocument verifies that an additional multi-doc DiscoveryServiceConfig is
// picked up by the discovery service controller and that discovery keeps working with multiple endpoints.
//
// The new DiscoveryServiceConfig document is mutually exclusive with the legacy cluster.discovery block
// (enforced by V1Alpha1ConflictValidate), so this scenario only applies to clusters using the new config.
func (suite *DiscoverySuite) TestDiscoveryServiceConfigDocument() {
	node := suite.RandomDiscoveredNodeInternalIP()
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)
	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if provider.Cluster().Discovery().Enabled() {
		suite.T().Skip("cluster uses the legacy v1alpha1 discovery config")
	}

	discoveryServiceConfigs := provider.DiscoveryServiceConfigs()
	if len(discoveryServiceConfigs) == 0 {
		suite.T().Skip("discovery service is disabled")
	}

	const extraName = "integration-extra"

	// reuse an existing endpoint so the extra discovery client connects to a real service;
	// multiple documents pointing at the same endpoint are allowed as long as their names differ
	cfgDocument := clustertypes.NewDiscoveryServiceConfigV1Alpha1(extraName, discoveryServiceConfigs[0].Endpoint())

	suite.T().Logf("injecting extra discovery service %q on node %s", extraName, node)
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	// the cluster Config resource should now include the extra named endpoint
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{cluster.ConfigID},
		func(cfg *cluster.Config, asrt *assert.Assertions) {
			asrt.Contains(xslices.Map(cfg.TypedSpec().ServiceEndpoints, func(ep cluster.ServiceEndpoint) string { return ep.Name }), extraName)
		},
		rtestutils.WithNamespace(resourcesconfig.NamespaceName),
	)

	// discovery should still work with multiple endpoints: all other nodes remain discovered as members
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Assert().Len(suite.getMembers(nodeCtx), len(nodes))

	suite.T().Logf("removing extra discovery service %q from node %s", extraName, node)
	suite.RemoveMachineConfigDocumentsByName(nodeCtx, clustertypes.DiscoveryServiceKind, extraName)

	// after removal, the extra endpoint disappears from the resource
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{cluster.ConfigID},
		func(cfg *cluster.Config, asrt *assert.Assertions) {
			asrt.NotContains(xslices.Map(cfg.TypedSpec().ServiceEndpoints, func(ep cluster.ServiceEndpoint) string { return ep.Name }), extraName)
		},
		rtestutils.WithNamespace(resourcesconfig.NamespaceName),
	)

	// removing one of several endpoints must not disrupt discovery: all nodes remain members
	rtestutils.AssertLength[*cluster.Member](nodeCtx, suite.T(), suite.Client.COSI, len(suite.DiscoverNodeInternalIPs(suite.ctx)))

	// Removing all discovery service configs disables discovery on this node entirely. On a KubeSpan
	// cluster the WireGuard peer set is populated from discovery, so disabling it drops all peers and
	// partitions the node from the cluster (on a control plane node this collapses etcd and the API VIP).
	// Skip the destructive check there; the multi-endpoint behavior above is the feature under test.
	if kubeSpan := provider.NetworkKubeSpanConfig(); kubeSpan != nil && kubeSpan.Enabled() {
		return
	}

	// guarantee the original configuration is restored even if an assertion below fails partway
	// through; re-applying the same documents is idempotent, so the explicit restore at the end of
	// the happy path is harmless.
	originalDocuments := xslices.Map(discoveryServiceConfigs, func(c configconfig.DiscoveryServiceConfig) any {
		return clustertypes.NewDiscoveryServiceConfigV1Alpha1(c.Name(), c.Endpoint())
	})

	defer func() {
		suite.T().Logf("ensuring original discovery service configuration is restored on node %s", node)
		suite.PatchMachineConfig(nodeCtx, originalDocuments...)
	}()

	suite.T().Logf("removing all discovery service configs from node %s", node)
	suite.RemoveMachineConfigDocuments(nodeCtx, clustertypes.DiscoveryServiceKind)

	// the cluster Config resource should no longer carry any service endpoints
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI, []string{cluster.ConfigID},
		func(cfg *cluster.Config, asrt *assert.Assertions) {
			asrt.Empty(cfg.TypedSpec().ServiceEndpoints)
		},
		rtestutils.WithNamespace(resourcesconfig.NamespaceName),
	)

	// with discovery disabled, no members should be discovered.
	rtestutils.AssertLength[*cluster.Member](nodeCtx, suite.T(), suite.Client.COSI, 0)

	// restore the original configuration and verify discovery reconverges: all other nodes are
	// discovered as members again.
	suite.T().Logf("restoring original discovery service configuration on node %s", node)
	suite.PatchMachineConfig(nodeCtx, originalDocuments...)

	rtestutils.AssertLength[*cluster.Member](nodeCtx, suite.T(), suite.Client.COSI, len(suite.DiscoverNodeInternalIPs(suite.ctx)))
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

	if kubeSpan := provider.NetworkKubeSpanConfig(); kubeSpan == nil || !kubeSpan.Enabled() {
		suite.T().Skip("KubeSpan is disabled")
	}

	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		rtestutils.AssertLength[*kubespan.PeerSpec](nodeCtx, suite.T(), suite.Client.COSI, len(nodes)-1)
		rtestutils.AssertLength[*kubespan.PeerStatus](nodeCtx, suite.T(), suite.Client.COSI, len(nodes)-1)

		rtestutils.AssertAll(nodeCtx, suite.T(), suite.Client.COSI,
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

	if kubeSpan := provider.NetworkKubeSpanConfig(); kubeSpan == nil || !kubeSpan.Enabled() {
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
	rtestutils.AssertResources(
		client.WithNode(suite.ctx, checkNode), suite.T(), suite.Client.COSI, []string{targetIdentity.TypedSpec().PublicKey},
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
