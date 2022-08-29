// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	clusteradapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/cluster"
	kubespanadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/kubespan"
	clusterctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubespan"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/version"
)

type LocalAffiliateSuite struct {
	ClusterSuite
}

func (suite *LocalAffiliateSuite) TestGeneration() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.LocalAffiliateController{}))

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

	nonK8sAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))
	nonK8sAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("172.20.0.2/24"),
		netip.MustParsePrefix("10.5.0.1/32"),
		netip.MustParsePrefix("fdae:41e4:649b:9303:60be:7e36:c270:3238/128"), // SideroLink, should be ignored
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, nonK8sAddresses))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			suite.Assert().Equal([]netip.Addr{netip.MustParseAddr("172.20.0.2"), netip.MustParseAddr("10.5.0.1")}, spec.Addresses)
			suite.Assert().Equal("example1", spec.Hostname)
			suite.Assert().Equal("example1.com", spec.Nodename)
			suite.Assert().Equal(machine.TypeWorker, spec.MachineType)
			suite.Assert().Equal("Talos ("+version.Tag+")", spec.OperatingSystem)
			suite.Assert().Equal(cluster.KubeSpanAffiliateSpec{}, spec.KubeSpan)

			return nil
		}),
	))

	// enable kubespan
	mac, err := net.ParseMAC("ea:71:1b:b2:cc:ee")
	suite.Require().NoError(err)

	ksIdentity := kubespan.NewIdentity(kubespan.NamespaceName, kubespan.LocalIdentity)
	suite.Require().NoError(kubespanadapter.IdentitySpec(ksIdentity.TypedSpec()).GenerateKey())
	suite.Require().NoError(kubespanadapter.IdentitySpec(ksIdentity.TypedSpec()).UpdateAddress("8XuV9TZHW08DOk3bVxQjH9ih_TBKjnh-j44tsCLSBzo=", mac))
	suite.Require().NoError(suite.state.Create(suite.ctx, ksIdentity))

	ksConfig := kubespan.NewConfig(config.NamespaceName, kubespan.ConfigID)
	ksConfig.TypedSpec().AdvertiseKubernetesNetworks = true
	suite.Require().NoError(suite.state.Create(suite.ctx, ksConfig))

	// add KS address to the list of node addresses, it should be ignored in the endpoints
	nonK8sAddresses.TypedSpec().Addresses = append(nonK8sAddresses.TypedSpec().Addresses, ksIdentity.TypedSpec().Address)
	suite.Require().NoError(suite.state.Update(suite.ctx, nonK8sAddresses))

	onlyK8sAddresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterOnlyK8s))
	onlyK8sAddresses.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.244.1.0/24")}
	suite.Require().NoError(suite.state.Create(suite.ctx, onlyK8sAddresses))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			if len(spec.Addresses) < 3 {
				return retry.ExpectedErrorf("not reconciled yet")
			}

			suite.Assert().Equal([]netip.Addr{netip.MustParseAddr("172.20.0.2"), netip.MustParseAddr("10.5.0.1"), ksIdentity.TypedSpec().Address.Addr()}, spec.Addresses)
			suite.Assert().Equal("example1", spec.Hostname)
			suite.Assert().Equal("example1.com", spec.Nodename)
			suite.Assert().Equal(machine.TypeWorker, spec.MachineType)

			if spec.KubeSpan.PublicKey == "" {
				return retry.ExpectedErrorf("kubespan is not filled in yet")
			}

			if spec.KubeSpan.AdditionalAddresses == nil {
				return retry.ExpectedErrorf("kubespan is not filled in yet")
			}

			suite.Assert().Equal(ksIdentity.TypedSpec().Address.Addr(), spec.KubeSpan.Address)
			suite.Assert().Equal(ksIdentity.TypedSpec().PublicKey, spec.KubeSpan.PublicKey)
			suite.Assert().Equal([]netip.Prefix{netip.MustParsePrefix("10.244.1.0/24")}, spec.KubeSpan.AdditionalAddresses)
			suite.Assert().Equal([]netip.AddrPort{netip.MustParseAddrPort("172.20.0.2:51820"), netip.MustParseAddrPort("10.5.0.1:51820")}, spec.KubeSpan.Endpoints)

			return nil
		}),
	))

	// disable advertising K8s addresses
	ksConfig.TypedSpec().AdvertiseKubernetesNetworks = false
	suite.Require().NoError(suite.state.Update(suite.ctx, ksConfig))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID).Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			if spec.KubeSpan.AdditionalAddresses != nil {
				return retry.ExpectedErrorf("additional addresses are not cleared yet")
			}

			suite.Assert().Empty(spec.KubeSpan.AdditionalAddresses)

			return nil
		}),
	))

	// disable discovery, local affiliate should be removed
	discoveryConfig.TypedSpec().DiscoveryEnabled = false
	suite.Require().NoError(suite.state.Update(suite.ctx, discoveryConfig))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertNoResource(*cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID).Metadata()),
	))
}

func TestLocalAffiliateSuite(t *testing.T) {
	suite.Run(t, new(LocalAffiliateSuite))
}
