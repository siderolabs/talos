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
	normalizedEndpoint, isInsecure, err := clusterctrl.NormalizeDiscoveryEndpoint(constants.DefaultDiscoveryServiceEndpoint)
	suite.Require().NoError(err)

	clusterIDRaw := make([]byte, constants.DefaultClusterIDSize)
	_, err = io.ReadFull(rand.Reader, clusterIDRaw)
	suite.Require().NoError(err)

	clusterID := base64.StdEncoding.EncodeToString(clusterIDRaw)

	encryptionKey := make([]byte, constants.DefaultClusterSecretSize)
	_, err = io.ReadFull(rand.Reader, encryptionKey)
	suite.Require().NoError(err)

	// regular discovery affiliate, registered against two discovery service endpoints
	// (both pointing at the same service, sharing the cluster ID and encryption key)
	clusterConf := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	clusterConf.TypedSpec().ServiceEndpoints = []cluster.ServiceEndpoint{
		{
			Name:     "default",
			Endpoint: normalizedEndpoint,
			Insecure: isInsecure,
		},
		{
			Name:     "secondary",
			Endpoint: normalizedEndpoint,
			Insecure: isInsecure,
		},
	}
	clusterConf.TypedSpec().ServiceClusterID = clusterID
	clusterConf.TypedSpec().ServiceEncryptionKey = encryptionKey
	suite.Create(clusterConf)

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
	cipher, err := aes.NewCipher(clusterConf.TypedSpec().ServiceEncryptionKey)
	suite.Require().NoError(err)

	cli, err := client.NewClient(client.Options{
		Cipher:      cipher,
		Endpoint:    normalizedEndpoint,
		ClusterID:   clusterConf.TypedSpec().ServiceClusterID,
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

	// drop the "secondary" endpoint: the controller should gracefully stop that client while the
	// "default" client keeps running and the discovered affiliate remains published.
	ctest.UpdateWithConflicts(suite, clusterConf, func(r *cluster.Config) error {
		r.TypedSpec().ServiceEndpoints = []cluster.ServiceEndpoint{
			{
				Name:     "default",
				Endpoint: normalizedEndpoint,
				Insecure: isInsecure,
			},
		}

		return nil
	})

	ctest.AssertResource(suite, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", func(r *cluster.Affiliate, asrt *assert.Assertions) {
		asrt.Equal("7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", r.TypedSpec().NodeID)
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
	normalizedEndpoint, isInsecure, err := clusterctrl.NormalizeDiscoveryEndpoint(constants.DefaultDiscoveryServiceEndpoint)
	suite.Require().NoError(err)

	clusterIDRaw := make([]byte, constants.DefaultClusterIDSize)
	_, err = io.ReadFull(rand.Reader, clusterIDRaw)
	suite.Require().NoError(err)

	clusterID := base64.StdEncoding.EncodeToString(clusterIDRaw)

	encryptionKey := make([]byte, constants.DefaultClusterSecretSize)
	_, err = io.ReadFull(rand.Reader, encryptionKey)
	suite.Require().NoError(err)

	// regular discovery affiliate
	discoveryConfig := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	discoveryConfig.TypedSpec().ServiceEndpoints = []cluster.ServiceEndpoint{
		{
			Name:     "default",
			Endpoint: normalizedEndpoint,
			Insecure: isInsecure,
		},
	}
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
		Endpoint:    normalizedEndpoint,
		Insecure:    isInsecure,
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
		r.TypedSpec().ServiceEndpoints = nil

		return nil
	})

	ctest.AssertNoResource[*cluster.Affiliate](suite, "service/7x1SuC8Ege5BGXdAfTEff5iQnlWZLfv9h1LGMxA2pYkC", rtestutils.WithNamespace(cluster.RawNamespaceName))

	cliCtxCancel()
	suite.Assert().NoError(<-errCh)
}

// TestEndpointChange verifies that changing a configured endpoint's value gracefully recreates the
// client against the new endpoint.
//
// There is no in-process discovery service to point a second endpoint at, and a registration only
// disappears from the real service after its TTL (30m), so this drives the observable direction:
// the endpoint starts unreachable (controller can't register anywhere), then flips to the real
// service, and we assert the affiliate shows up — which only happens if the recreated client
// connected to the new endpoint.
func (suite *DiscoveryServiceSuite) TestEndpointChange() {
	normalizedEndpoint, isInsecure, err := clusterctrl.NormalizeDiscoveryEndpoint(constants.DefaultDiscoveryServiceEndpoint)
	suite.Require().NoError(err)

	clusterIDRaw := make([]byte, constants.DefaultClusterIDSize)
	_, err = io.ReadFull(rand.Reader, clusterIDRaw)
	suite.Require().NoError(err)

	clusterID := base64.StdEncoding.EncodeToString(clusterIDRaw)

	encryptionKey := make([]byte, constants.DefaultClusterSecretSize)
	_, err = io.ReadFull(rand.Reader, encryptionKey)
	suite.Require().NoError(err)

	// start with a single endpoint pointing at an unreachable address: the controller cannot register
	// its affiliate anywhere until the endpoint is changed.
	endpointUnreachable := cluster.ServiceEndpoint{
		Name:     "default",
		Endpoint: "127.0.0.1:1",
		Insecure: true,
	}
	discoveryConfig := cluster.NewConfig(config.NamespaceName, cluster.ConfigID)
	discoveryConfig.TypedSpec().ServiceEndpoints = []cluster.ServiceEndpoint{endpointUnreachable}
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

	// create a test client connected to the real service under a different affiliate ID
	cipher, err := aes.NewCipher(discoveryConfig.TypedSpec().ServiceEncryptionKey)
	suite.Require().NoError(err)

	cli, err := client.NewClient(client.Options{
		Cipher:      cipher,
		Endpoint:    normalizedEndpoint,
		Insecure:    isInsecure,
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

	// while the controller's only endpoint is unreachable, it cannot register: the test client (on the
	// real service) should not discover the local affiliate. Best-effort negative check.
	time.Sleep(2 * time.Second)
	suite.Assert().Empty(cli.GetAffiliates())

	// change the endpoint to the real service: the controller should recreate the client against it.
	ctest.UpdateWithConflicts(suite, discoveryConfig, func(r *cluster.Config) error {
		r.TypedSpec().ServiceEndpoints = []cluster.ServiceEndpoint{
			{
				Name:     "default",
				Endpoint: normalizedEndpoint,
				Insecure: isInsecure,
			},
		}

		return nil
	})

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		// the recreated client connects to the new endpoint and registers, so we should see it discovered
		affiliates := cli.GetAffiliates()

		if len(affiliates) != 1 {
			return retry.ExpectedErrorf("affiliates len %d != 1", len(affiliates))
		}

		if affiliates[0].Affiliate.NodeId != nodeIdentity.TypedSpec().NodeID {
			return retry.ExpectedErrorf("unexpected node ID %q", affiliates[0].Affiliate.NodeId)
		}

		return nil
	})

	cliCtxCancel()
	suite.Assert().NoError(<-errCh)
}

func TestNormalizeDiscoveryEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		expectErr      bool
		expectAddr     string
		expectInsecure bool
	}{
		{
			name:           "HTTPS with explicit port",
			endpoint:       "https://discovery.example.com:6443",
			expectErr:      false,
			expectAddr:     "discovery.example.com:6443",
			expectInsecure: false,
		},
		{
			name:           "HTTPS without port defaults to 443",
			endpoint:       "https://discovery.example.com",
			expectErr:      false,
			expectAddr:     "discovery.example.com:443",
			expectInsecure: false,
		},
		{
			name:           "HTTP with explicit port",
			endpoint:       "http://discovery.example.com:8080",
			expectErr:      false,
			expectAddr:     "discovery.example.com:8080",
			expectInsecure: true,
		},
		{
			name:           "HTTP without port defaults to 80",
			endpoint:       "http://discovery.example.com",
			expectErr:      false,
			expectAddr:     "discovery.example.com:80",
			expectInsecure: true,
		},
		{
			name:           "HTTPS without port defaults to 443",
			endpoint:       "https://discovery.example.com",
			expectErr:      false,
			expectAddr:     "discovery.example.com:443",
			expectInsecure: false,
		},
		{
			name:           "HTTPS with IPv4 and port",
			endpoint:       "https://192.168.1.1:6443",
			expectErr:      false,
			expectAddr:     "192.168.1.1:6443",
			expectInsecure: false,
		},
		{
			name:           "HTTP with IPv4 defaults to port 80",
			endpoint:       "http://192.168.1.1",
			expectErr:      false,
			expectAddr:     "192.168.1.1:80",
			expectInsecure: true,
		},
		{
			name:           "HTTPS with IPv6 and port",
			endpoint:       "https://[::1]:6443",
			expectErr:      false,
			expectAddr:     "[::1]:6443",
			expectInsecure: false,
		},
		{
			name:           "HTTP with IPv6 defaults to port 80",
			endpoint:       "http://[::1]",
			expectErr:      false,
			expectAddr:     "[::1]:80",
			expectInsecure: true,
		},
		{
			name:           "HTTPS with path is stripped",
			endpoint:       "https://discovery.example.com:6443/api/v1",
			expectErr:      false,
			expectAddr:     "discovery.example.com:6443",
			expectInsecure: false,
		},
		{
			name:           "HTTPS with query string is stripped",
			endpoint:       "https://discovery.example.com:6443?key=value",
			expectErr:      false,
			expectAddr:     "discovery.example.com:6443",
			expectInsecure: false,
		},
		{
			name:           "Invalid URL returns error",
			endpoint:       "not a valid url://[",
			expectErr:      true,
			expectAddr:     "",
			expectInsecure: false,
		},
		{
			name:      "Empty URL returns error",
			endpoint:  "",
			expectErr: true,
		},
		{
			name:      "Scheme-less URL returns error",
			endpoint:  "discovery.example.com",
			expectErr: true,
		},
		{
			name:      "Unsupported scheme returns error",
			endpoint:  "ftp://discovery.example.com",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, insecure, err := clusterctrl.NormalizeDiscoveryEndpoint(tt.endpoint)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if addr != tt.expectAddr {
				t.Errorf("expected addr %q, got %q", tt.expectAddr, addr)
			}

			if insecure != tt.expectInsecure {
				t.Errorf("expected insecure %v, got %v", tt.expectInsecure, insecure)
			}
		})
	}
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
