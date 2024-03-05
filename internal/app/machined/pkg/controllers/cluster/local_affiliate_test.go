// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net"
	"net/netip"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

type LocalAffiliateSuite struct {
	ClusterSuite
}

func (suite *LocalAffiliateSuite) TestGeneration() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.LocalAffiliateController{}))

	nodeIdentity, nonK8sRoutedAddresses, discoveryConfig := suite.createResources()

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	ctest.AssertResource(suite, nodeIdentity.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal([]string{
			"172.20.0.2",
			"10.5.0.1",
			"192.168.192.168",
			"2001:123:4567::1",
		}, xslices.Map(spec.Addresses, netip.Addr.String))
		asrt.Equal("example1", spec.Hostname)
		asrt.Equal("example1.com", spec.Nodename)
		asrt.Equal(machine.TypeWorker, spec.MachineType)
		asrt.Equal("Talos ("+version.Tag+")", spec.OperatingSystem)
		asrt.Equal(cluster.KubeSpanAffiliateSpec{}, spec.KubeSpan)
	})

	// enable kubespan
	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	ksIdentity := kubespan.NewIdentity(kubespan.NamespaceName, kubespan.LocalIdentity)
	suite.Require().NoError(kubespanadapter.IdentitySpec(ksIdentity.TypedSpec()).GenerateKey())
	suite.Require().NoError(kubespanadapter.IdentitySpec(ksIdentity.TypedSpec()).UpdateAddress("8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=", mac))
	suite.Require().NoError(suite.state.Create(suite.ctx, ksIdentity))

	ksConfig := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	ksConfig.TypedSpec().EndpointFilters = []string{"0.0.0.0/0", "!192.168.0.0/16", "2001::/16"}
	ksConfig.TypedSpec().AdvertiseKubernetesNetworks = true
	suite.Require().NoError(suite.state.Create(suite.ctx, ksConfig))

	// add KS address to the list of node addresses, it should be ignored in the endpoints
	nonK8sRoutedAddresses.TypedSpec().Addresses = append(nonK8sRoutedAddresses.TypedSpec().Addresses, ksIdentity.TypedSpec().Address)
	suite.Require().NoError(suite.state.Update(suite.ctx, nonK8sRoutedAddresses))

	onlyK8sAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterOnlyK8s))
	onlyK8sAddresses.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.244.1.0/24")}
	suite.Require().NoError(suite.state.Create(suite.ctx, onlyK8sAddresses))

	// add discovered public IPs
	for _, addr := range []netip.Addr{
		netip.MustParseAddr("1.1.1.1"),
		netip.MustParseAddr("2001:123:4567::1"), // duplicate, will be ignored
	} {
		discoveredAddr := network.NewAddressStatus(cluster.NamespaceName, addr.String())
		discoveredAddr.TypedSpec().Address = netip.PrefixFrom(addr, addr.BitLen())
		suite.Require().NoError(suite.state.Create(suite.ctx, discoveredAddr))
	}

	ctest.AssertResource(suite, nodeIdentity.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.False(len(spec.Addresses) < 5)

		asrt.Equal([]netip.Addr{
			netip.MustParseAddr("172.20.0.2"),
			netip.MustParseAddr("10.5.0.1"),
			netip.MustParseAddr("192.168.192.168"),
			netip.MustParseAddr("2001:123:4567::1"),
			ksIdentity.TypedSpec().Address.Addr(),
		}, spec.Addresses)

		asrt.Equal("example1", spec.Hostname)
		asrt.Equal("example1.com", spec.Nodename)
		asrt.Equal(machine.TypeWorker, spec.MachineType)

		asrt.NotZero(spec.KubeSpan.PublicKey)
		asrt.NotZero(spec.KubeSpan.AdditionalAddresses)
		asrt.Len(spec.KubeSpan.Endpoints, 4)

		asrt.Equal(ksIdentity.TypedSpec().Address.Addr(), spec.KubeSpan.Address)
		asrt.Equal(ksIdentity.TypedSpec().PublicKey, spec.KubeSpan.PublicKey)
		asrt.Equal([]netip.Prefix{netip.MustParsePrefix("10.244.1.0/24")}, spec.KubeSpan.AdditionalAddresses)
		asrt.Equal(
			[]string{
				"172.20.0.2:51820",
				"10.5.0.1:51820",
				"1.1.1.1:51820",
				"[2001:123:4567::1]:51820",
			},
			xslices.Map(spec.KubeSpan.Endpoints, netip.AddrPort.String),
		)
	})

	// disable advertising K8s addresses
	ksConfig.TypedSpec().AdvertiseKubernetesNetworks = false
	suite.Require().NoError(suite.state.Update(suite.ctx, ksConfig))

	ctest.AssertResource(suite, nodeIdentity.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().KubeSpan.AdditionalAddresses)
	})

	// disable discovery, local affiliate should be removed
	discoveryConfig.TypedSpec().DiscoveryEnabled = false
	suite.Require().NoError(suite.state.Update(suite.ctx, discoveryConfig))

	ctest.AssertNoResource[*cluster.Affiliate](suite, nodeIdentity.TypedSpec().NodeID)
}

func (suite *LocalAffiliateSuite) TestCPGeneration() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.LocalAffiliateController{}))

	nodeIdentity, _, discoveryConfig := suite.createResources()

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	apiServerConfig := k8s.NewAPIServerConfig()
	apiServerConfig.TypedSpec().LocalPort = 6445
	suite.Require().NoError(suite.state.Create(suite.ctx, apiServerConfig))

	ctest.AssertResource(suite, nodeIdentity.TypedSpec().NodeID, func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal([]string{
			"172.20.0.2",
			"10.5.0.1",
			"192.168.192.168",
			"2001:123:4567::1",
		}, xslices.Map(spec.Addresses, netip.Addr.String))
		asrt.Equal("example1", spec.Hostname)
		asrt.Equal("example1.com", spec.Nodename)
		asrt.Equal(machine.TypeControlPlane, spec.MachineType)
		asrt.Equal("Talos ("+version.Tag+")", spec.OperatingSystem)
		asrt.Equal(cluster.KubeSpanAffiliateSpec{}, spec.KubeSpan)
		asrt.NotNil(spec.ControlPlane)
		asrt.Equal(6445, spec.ControlPlane.APIServerPort)
	})

	discoveryConfig.TypedSpec().DiscoveryEnabled = false
	suite.Require().NoError(suite.state.Update(suite.ctx, discoveryConfig))

	ctest.AssertNoResource[*cluster.Affiliate](suite, nodeIdentity.TypedSpec().NodeID)
}

func (suite *LocalAffiliateSuite) createResources() (*cluster.Identity, *network.NodeAddress, *cluster.Config) {
	// regular discovery affiliate
	discoveryConfig := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	discoveryConfig.TypedSpec().DiscoveryEnabled = true
	suite.Require().NoError(suite.state.Create(suite.ctx, discoveryConfig))

	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(clusteradapter.IdentitySpec(nodeIdentity.TypedSpec()).Generate())
	suite.Require().NoError(suite.state.Create(suite.ctx, nodeIdentity))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "example1"
	suite.Require().NoError(suite.state.Create(suite.ctx, hostnameStatus))

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example1.com"
	suite.Require().NoError(suite.state.Create(suite.ctx, nodename))

	nonK8sCurrentAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))
	nonK8sCurrentAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("172.20.0.2/24"),
		netip.MustParsePrefix("10.5.0.1/32"),
		netip.MustParsePrefix("192.168.192.168/24"),
		netip.MustParsePrefix("2001:123:4567::1/64"),
		netip.MustParsePrefix("2001:123:4567::1/128"),
		netip.MustParsePrefix("fdae:41e4:649b:9303:60be:7e36:c270:3238/128"), // SideroLink, should be ignored
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, nonK8sCurrentAddresses))

	nonK8sRoutedAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s))
	nonK8sRoutedAddresses.TypedSpec().Addresses = []netip.Prefix{ // routed node addresses don't contain SideroLink addresses
		netip.MustParsePrefix("172.20.0.2/24"),
		netip.MustParsePrefix("10.5.0.1/32"),
		netip.MustParsePrefix("192.168.192.168/24"),
		netip.MustParsePrefix("2001:123:4567::1/64"),
		netip.MustParsePrefix("2001:123:4567::1/128"),
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, nonK8sRoutedAddresses))

	return nodeIdentity, nonK8sRoutedAddresses, discoveryConfig
}

func TestLocalAffiliateSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(LocalAffiliateSuite))
}
