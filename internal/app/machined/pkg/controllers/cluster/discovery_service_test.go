// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/discovery-api/api/v1alpha1/client/pb"
	"github.com/siderolabs/discovery-client/pkg/client"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	clusterctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type DiscoveryServiceSuite struct {
	ctest.DefaultSuite
}

func (suite *DiscoveryServiceSuite) TestReconcile() {
	serviceEndpoint, err := url.Parse(constants.DefaultDiscoveryServiceEndpoint)
	suite.Require().NoError(err)

	if serviceEndpoint.Port() == "" {
		serviceEndpoint.Host += ":443"
	}

	clusterIDRaw := make([]byte, constants.DefaultClusterIDSize)
	_, err = io.ReadFull(rand.Reader, clusterIDRaw)
	suite.Require().NoError(err)

	clusterID := base64.StdEncoding.EncodeToString(clusterIDRaw)

	encryptionKey := make([]byte, constants.DefaultClusterSecretSize)
	_, err = io.ReadFull(rand.Reader, encryptionKey)
	suite.Require().NoError(err)

	// regular discovery affiliate
	discoveryConfig := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	discoveryConfig.TypedSpec().DiscoveryEnabled = true
	discoveryConfig.TypedSpec().RegistryServiceEnabled = true
	discoveryConfig.TypedSpec().ServiceEndpoint = serviceEndpoint.Host
	discoveryConfig.TypedSpec().ServiceClusterID = clusterID
	discoveryConfig.TypedSpec().ServiceEncryptionKey = encryptionKey
	suite.Create(discoveryConfig)

	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(clusteradapter.IdentitySpec(nodeIdentity.TypedSpec()).Generate())
	suite.Create(nodeIdentity)

	localAffiliate := cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID)
	*localAffiliate.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      nodeIdentity.TypedSpec().NodeID,
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.4")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:                 "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:                   netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses:       []netip.Prefix{netip.MustParsePrefix("10.244.3.1/24")},
			Endpoints:                 []netip.AddrPort{netip.MustParseAddrPort("10.0.0.2:51820"), netip.MustParseAddrPort("192.168.3.4:51820")},
			ExcludeAdvertisedNetworks: []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")},
		},
		ControlPlane: &cluster.ControlPlane{APIServerPort: 6443},
	}
	suite.Create(localAffiliate)

	// create a test client connected to the same cluster but under different affiliate ID
	cipher, err := aes.NewCipher(discoveryConfig.TypedSpec().ServiceEncryptionKey)
	suite.Require().NoError(err)

	cli, err := client.NewClient(client.Options{
		Cipher:      cipher,
		Endpoint:    serviceEndpoint.Host,
		ClusterID:   discoveryConfig.TypedSpec().ServiceClusterID,
		AffiliateID: "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		TTL:         5 * time.Minute,
	})
	suite.Require().NoError(err)

	errCh := make(chan error, 1)
	notifyCh := make(chan struct{}, 1)

	cliCtx, cliCtxCancel := context.WithCancel(suite.Ctx())
	defer cliCtxCancel()

	go func() {
		errCh <- cli.Run(cliCtx, zaptest.NewLogger(suite.T()), notifyCh)
	}()

	suite.AssertWithin(3*time.Second, 100*time.Millisecond, func() error {
		// controller should register its local affiliate, and we should see it being discovered
		affiliates := cli.GetAffiliates()

		if len(affiliates) != 1 {
			return retry.ExpectedErrorf("affiliates len %d != 1", len(affiliates))
		}

		suite.Require().Len(affiliates[0].Endpoints, 2)
		suite.Assert().True(proto.Equal(&pb.Affiliate{
			NodeId:          nodeIdentity.TypedSpec().NodeID,
			Addresses:       [][]byte{[]byte("\xc0\xa8\x03\x04")},
			Hostname:        "foo.com",
			Nodename:        "bar",
			MachineType:     "controlplane",
			OperatingSystem: "",
			Kubespan: &pb.KubeSpan{
				PublicKey: "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
				Address:   []byte("\xfd\x50\x8d\x60\x42\x38\x63\x02\xf8\x57\x23\xff\xfe\x21\xd1\xe0"),
				AdditionalAddresses: []*pb.IPPrefix{
					{
						Ip:   []byte("\x0a\xf4\x03\x01"),
						Bits: 24,
					},
				},
				ExcludeAdvertisedAddresses: []*pb.IPPrefix{
					{
						Ip:   []byte("\x00\x00\x00\x00"),
						Bits: 0,
					},
				},
			},
			ControlPlane: &pb.ControlPlane{ApiServerPort: 6443},
		}, affiliates[0].Affiliate))
		suite.Assert().True(proto.Equal(
			&pb.Endpoint{
				Ip:   []byte("\n\x00\x00\x02"),
				Port: 51820,
			},
			affiliates[0].Endpoints[0],
		), "expected %v", affiliates[0].Endpoints[0])
		suite.Assert().True(proto.Equal(
			&pb.Endpoint{
				Ip:   []byte("\xc0\xa8\x03\x04"),
				Port: 51820,
			},
			affiliates[0].Endpoints[1],
		), "expected %v", affiliates[0].Endpoints[1])

		return nil
	})

	// inject some affiliate via our client, controller should publish it as an affiliate
	suite.Require().NoError(cli.SetLocalData(&client.Affiliate{
		Affiliate: &pb.Affiliate{
			NodeId:          "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
			Addresses:       [][]byte{[]byte("\xc0\xa8\x03\x05")},
			Hostname:        "some.com",
			Nodename:        "some",
			MachineType:     "worker",
			OperatingSystem: "test OS",
			Kubespan: &pb.KubeSpan{
				PublicKey: "1CXkdhWBm58c36kTpchR8iGlXHG1ruHa5W8gsFqD8Qs=",
				Address:   []byte("\xfd\x50\x8d\x60\x42\x38\x63\x02\xf8\x57\x23\xff\xfe\x21\xd1\xe1"),
				AdditionalAddresses: []*pb.IPPrefix{
					{
						Ip:   []byte("\x0a\xf4\x04\x01"),
						Bits: 24,
					},
				},
				ExcludeAdvertisedAddresses: []*pb.IPPrefix{
					{
						Ip:   []byte("\x01\x01\x01\x01"),
						Bits: 32,
					},
				},
			},
		},
		Endpoints: []*pb.Endpoint{
			{
				Ip:   []byte("\xc0\xa8\x03\x05"),
				Port: 51820,
			},
		},
	}, nil))

	ctest.AssertResource(suite, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal("7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", spec.NodeID)
		asrt.Equal([]netip.Addr{netip.MustParseAddr("192.168.3.5")}, spec.Addresses)
		asrt.Equal("some.com", spec.Hostname)
		asrt.Equal("some", spec.Nodename)
		asrt.Equal(machine.TypeWorker, spec.MachineType)
		asrt.Equal("test OS", spec.OperatingSystem)
		asrt.Equal(netip.MustParseAddr("fd50:8d60:4238:6302:f857:23ff:fe21:d1e1"), spec.KubeSpan.Address)
		asrt.Equal("1CXkdhWBm58c36kTpchR8iGlXHG1ruHa5W8gsFqD8Qs=", spec.KubeSpan.PublicKey)
		asrt.Equal([]netip.Prefix{netip.MustParsePrefix("10.244.4.1/24")}, spec.KubeSpan.AdditionalAddresses)
		asrt.Equal([]netip.Prefix{netip.MustParsePrefix("1.1.1.1/32")}, spec.KubeSpan.ExcludeAdvertisedNetworks)
		asrt.Equal([]netip.AddrPort{netip.MustParseAddrPort("192.168.3.5:51820")}, spec.KubeSpan.Endpoints)
		asrt.Zero(spec.ControlPlane)
	}, rtestutils.WithNamespace(cluster.RawNamespaceName))

	// controller should publish public IP
	ctest.AssertResource(suite, "service", func(r *network.AddressStatus, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.True(spec.Address.IsValid())
		asrt.True(spec.Address.IsSingleIP())
	}, rtestutils.WithNamespace(cluster.NamespaceName))

	// make controller inject additional endpoint via kubespan.Endpoint
	endpoint := kubespan.NewEndpoint(kubespan.NamespaceName, "1CXkdhWBm58c36kTpchR8iGlXHG1ruHa5W8gsFqD8Qs=")
	*endpoint.TypedSpec() = kubespan.EndpointSpec{
		AffiliateID: "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Endpoint:    netip.MustParseAddrPort("1.1.1.1:343"),
	}
	suite.Create(endpoint)

	ctest.AssertResource(suite, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", func(r *cluster.Affiliate, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Len(spec.KubeSpan.Endpoints, 2)
		asrt.Equal([]netip.AddrPort{
			netip.MustParseAddrPort("192.168.3.5:51820"),
			netip.MustParseAddrPort("1.1.1.1:343"),
		}, spec.KubeSpan.Endpoints)
	}, rtestutils.WithNamespace(cluster.RawNamespaceName))

	// pretend that machine is being reset
	suite.Create(runtime.NewMachineResetSignal())

	// client should see the affiliate being deleted
	suite.AssertWithin(3*time.Second, 100*time.Millisecond, func() error {
		// controller should delete its local affiliate
		affiliates := cli.GetAffiliates()

		if len(affiliates) != 0 {
			return retry.ExpectedErrorf("affiliates len %d != 0", len(affiliates))
		}

		return nil
	})

	cliCtxCancel()
	suite.Assert().NoError(<-errCh)
}

func (suite *DiscoveryServiceSuite) TestDisable() {
	serviceEndpoint, err := url.Parse(constants.DefaultDiscoveryServiceEndpoint)
	suite.Require().NoError(err)

	if serviceEndpoint.Port() == "" {
		serviceEndpoint.Host += ":443"
	}

	clusterIDRaw := make([]byte, constants.DefaultClusterIDSize)
	_, err = io.ReadFull(rand.Reader, clusterIDRaw)
	suite.Require().NoError(err)

	clusterID := base64.StdEncoding.EncodeToString(clusterIDRaw)

	encryptionKey := make([]byte, constants.DefaultClusterSecretSize)
	_, err = io.ReadFull(rand.Reader, encryptionKey)
	suite.Require().NoError(err)

	// regular discovery affiliate
	discoveryConfig := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	discoveryConfig.TypedSpec().DiscoveryEnabled = true
	discoveryConfig.TypedSpec().RegistryServiceEnabled = true
	discoveryConfig.TypedSpec().ServiceEndpoint = serviceEndpoint.Host
	discoveryConfig.TypedSpec().ServiceClusterID = clusterID
	discoveryConfig.TypedSpec().ServiceEncryptionKey = encryptionKey
	suite.Create(discoveryConfig)

	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(clusteradapter.IdentitySpec(nodeIdentity.TypedSpec()).Generate())
	suite.Create(nodeIdentity)

	localAffiliate := cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID)
	*localAffiliate.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      nodeIdentity.TypedSpec().NodeID,
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netip.Addr{netip.MustParseAddr("192.168.3.4")},
	}
	suite.Create(localAffiliate)

	// create a test client connected to the same cluster but under different affiliate ID
	cipher, err := aes.NewCipher(discoveryConfig.TypedSpec().ServiceEncryptionKey)
	suite.Require().NoError(err)

	cli, err := client.NewClient(client.Options{
		Cipher:      cipher,
		Endpoint:    serviceEndpoint.Host,
		ClusterID:   discoveryConfig.TypedSpec().ServiceClusterID,
		AffiliateID: "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		TTL:         5 * time.Minute,
	})
	suite.Require().NoError(err)

	errCh := make(chan error, 1)
	notifyCh := make(chan struct{}, 1)

	cliCtx, cliCtxCancel := context.WithCancel(suite.Ctx())
	defer cliCtxCancel()

	go func() {
		errCh <- cli.Run(cliCtx, zaptest.NewLogger(suite.T()), notifyCh)
	}()

	// inject some affiliate via our client, controller should publish it as an affiliate
	suite.Require().NoError(cli.SetLocalData(&client.Affiliate{
		Affiliate: &pb.Affiliate{
			NodeId: "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		},
	}, nil))

	ctest.AssertResource(suite, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", func(r *cluster.Affiliate, asrt *assert.Assertions) {
		asrt.Equal("7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", r.TypedSpec().NodeID)
	}, rtestutils.WithNamespace(cluster.RawNamespaceName))

	// now disable the service registry
	ctest.UpdateWithConflicts(suite, discoveryConfig, func(r *cluster.Config) error {
		r.TypedSpec().RegistryServiceEnabled = false

		return nil
	})

	ctest.AssertNoResource[*cluster.Affiliate](suite, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", rtestutils.WithNamespace(cluster.RawNamespaceName))

	cliCtxCancel()
	suite.Assert().NoError(<-errCh)
}

func TestDiscoveryServiceSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &DiscoveryServiceSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&clusterctrl.DiscoveryServiceController{}))
			},
		},
	})
}
