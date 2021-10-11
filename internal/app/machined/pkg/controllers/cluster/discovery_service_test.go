// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"log"
	"net"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/discovery-service/api/v1alpha1/client/pb"
	serverpb "github.com/talos-systems/discovery-service/api/v1alpha1/server/pb"
	"github.com/talos-systems/discovery-service/pkg/client"
	"github.com/talos-systems/discovery-service/pkg/server"
	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/grpc"
	"inet.af/netaddr"

	clusterctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/cluster"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/proto"
	"github.com/talos-systems/talos/pkg/resources/cluster"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/kubespan"
)

type DiscoveryServiceSuite struct {
	ClusterSuite
}

func setupServer(t *testing.T) (address string) {
	t.Helper()

	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	logger := logging.Wrap(log.Writer())

	serverOptions := []grpc.ServerOption{
		grpc_middleware.WithUnaryServerChain(
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(server.FieldExtractor)),
			grpc_zap.UnaryServerInterceptor(logger),
		),
		grpc_middleware.WithStreamServerChain(
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(server.FieldExtractor)),
			grpc_zap.StreamServerInterceptor(logger),
		),
	}

	s := grpc.NewServer(serverOptions...)
	serverpb.RegisterClusterServer(s, server.NewTestClusterServer(logger))

	go func() {
		require.NoError(t, s.Serve(lis))
	}()

	t.Cleanup(s.Stop)

	return lis.Addr().String()
}

func (suite *DiscoveryServiceSuite) TestReconcile() {
	suite.startRuntime()

	suite.Require().NoError(suite.runtime.RegisterController(&clusterctrl.DiscoveryServiceController{}))

	address := setupServer(suite.T())

	// regular discovery affiliate
	discoveryConfig := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	discoveryConfig.TypedSpec().DiscoveryEnabled = true
	discoveryConfig.TypedSpec().RegistryServiceEnabled = true
	discoveryConfig.TypedSpec().ServiceEndpoint = address
	discoveryConfig.TypedSpec().ServiceEndpointInsecure = true
	discoveryConfig.TypedSpec().ServiceClusterID = "fake"
	discoveryConfig.TypedSpec().ServiceEncryptionKey = bytes.Repeat([]byte{1}, 32)
	suite.Require().NoError(suite.state.Create(suite.ctx, discoveryConfig))

	nodeIdentity := cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity)
	suite.Require().NoError(nodeIdentity.TypedSpec().Generate())
	suite.Require().NoError(suite.state.Create(suite.ctx, nodeIdentity))

	localAffiliate := cluster.NewAffiliate(cluster.NamespaceName, nodeIdentity.TypedSpec().NodeID)
	*localAffiliate.TypedSpec() = cluster.AffiliateSpec{
		NodeID:      nodeIdentity.TypedSpec().NodeID,
		Hostname:    "foo.com",
		Nodename:    "bar",
		MachineType: machine.TypeControlPlane,
		Addresses:   []netaddr.IP{netaddr.MustParseIP("192.168.3.4")},
		KubeSpan: cluster.KubeSpanAffiliateSpec{
			PublicKey:           "PLPNBddmTgHJhtw0vxltq1ZBdPP9RNOEUd5JjJZzBRY=",
			Address:             netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e0"),
			AdditionalAddresses: []netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.3.1/24")},
			Endpoints:           []netaddr.IPPort{netaddr.MustParseIPPort("10.0.0.2:51820"), netaddr.MustParseIPPort("192.168.3.4:51820")},
		},
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, localAffiliate))

	// create a test client connected to the same cluster but under different affiliate ID
	cipher, err := aes.NewCipher(discoveryConfig.TypedSpec().ServiceEncryptionKey)
	suite.Require().NoError(err)

	cli, err := client.NewClient(client.Options{
		Cipher:      cipher,
		Endpoint:    address,
		ClusterID:   discoveryConfig.TypedSpec().ServiceClusterID,
		AffiliateID: "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		TTL:         5 * time.Minute,
		Insecure:    true,
	})
	suite.Require().NoError(err)

	errCh := make(chan error, 1)
	notifyCh := make(chan struct{}, 1)

	cliCtx, cliCtxCancel := context.WithCancel(suite.ctx)
	defer cliCtxCancel()

	go func() {
		errCh <- cli.Run(cliCtx, logging.Wrap(log.Writer()), notifyCh)
	}()

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
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
				},
			}, affiliates[0].Affiliate))
			suite.Assert().True(proto.Equal(
				&pb.Endpoint{
					Ip:   []byte("\n\x00\x00\x02"),
					Port: 51820,
				},
				affiliates[0].Endpoints[0]), "expected %v", affiliates[0].Endpoints[0])
			suite.Assert().True(proto.Equal(
				&pb.Endpoint{
					Ip:   []byte("\xc0\xa8\x03\x04"),
					Port: 51820,
				},
				affiliates[0].Endpoints[1]), "expected %v", affiliates[0].Endpoints[1])

			return nil
		},
	))

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
			},
		},
		Endpoints: []*pb.Endpoint{
			{
				Ip:   []byte("\xc0\xa8\x03\x05"),
				Port: 51820,
			},
		},
	}, nil))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.RawNamespaceName, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC").Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			suite.Assert().Equal("7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", spec.NodeID)
			suite.Assert().Equal([]netaddr.IP{netaddr.MustParseIP("192.168.3.5")}, spec.Addresses)
			suite.Assert().Equal("some.com", spec.Hostname)
			suite.Assert().Equal("some", spec.Nodename)
			suite.Assert().Equal(machine.TypeWorker, spec.MachineType)
			suite.Assert().Equal("test OS", spec.OperatingSystem)
			suite.Assert().Equal(netaddr.MustParseIP("fd50:8d60:4238:6302:f857:23ff:fe21:d1e1"), spec.KubeSpan.Address)
			suite.Assert().Equal("1CXkdhWBm58c36kTpchR8iGlXHG1ruHa5W8gsFqD8Qs=", spec.KubeSpan.PublicKey)
			suite.Assert().Equal([]netaddr.IPPrefix{netaddr.MustParseIPPrefix("10.244.4.1/24")}, spec.KubeSpan.AdditionalAddresses)
			suite.Assert().Equal([]netaddr.IPPort{netaddr.MustParseIPPort("192.168.3.5:51820")}, spec.KubeSpan.Endpoints)

			return nil
		}),
	))

	// make controller inject additional endpoint via kubespan.Endpoint
	endpoint := kubespan.NewEndpoint(kubespan.NamespaceName, "1CXkdhWBm58c36kTpchR8iGlXHG1ruHa5W8gsFqD8Qs=")
	*endpoint.TypedSpec() = kubespan.EndpointSpec{
		AffiliateID: "7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC",
		Endpoint:    netaddr.MustParseIPPort("1.1.1.1:343"),
	}
	suite.Require().NoError(suite.state.Create(suite.ctx, endpoint))

	suite.Assert().NoError(retry.Constant(3*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(*cluster.NewAffiliate(cluster.RawNamespaceName, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC").Metadata(), func(r resource.Resource) error {
			spec := r.(*cluster.Affiliate).TypedSpec()

			if len(spec.KubeSpan.Endpoints) != 2 {
				return retry.ExpectedErrorf("waiting for 2 endpoints, got %d", len(spec.KubeSpan.Endpoints))
			}

			suite.Assert().Equal([]netaddr.IPPort{
				netaddr.MustParseIPPort("192.168.3.5:51820"),
				netaddr.MustParseIPPort("1.1.1.1:343"),
			}, spec.KubeSpan.Endpoints)

			return nil
		}),
	))

	cliCtxCancel()
	suite.Assert().NoError(<-errCh)
}

func TestDiscoveryServiceSuite(t *testing.T) {
	suite.Run(t, new(DiscoveryServiceSuite))
}
