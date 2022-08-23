// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	pb "github.com/talos-systems/siderolink/api/siderolink"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/ctest"
	siderolinkctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/siderolink"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestManagerSuite(t *testing.T) {
	var m ManagerSuite
	m.AfterSetup = func(suite *ctest.DefaultSuite) {
		lis, err := net.Listen("tcp", "localhost:0")
		suite.Require().NoError(err)

		m.s = grpc.NewServer()
		pb.RegisterProvisionServiceServer(m.s, mockServer{})

		go func() {
			suite.Require().NoError(m.s.Serve(lis))
		}()

		cmdline := procfs.NewCmdline(fmt.Sprintf("%s=%s", constants.KernelParamSideroLink, lis.Addr().String()))

		suite.Require().NoError(suite.Runtime().RegisterController(&siderolinkctrl.ManagerController{
			Cmdline: cmdline,
		}))
	}

	suite.Run(t, &m)
}

type ManagerSuite struct {
	ctest.DefaultSuite
	s *grpc.Server
}

type mockServer struct {
	pb.UnimplementedProvisionServiceServer
}

const (
	mockServerEndpoint    = "127.0.0.11:51820"
	mockServerAddress     = "fdae:41e4:649b:9303:b6db:d99c:215e:dfc4"
	mockServerPublicKey   = "2aq/V91QyrHAoH24RK0bldukgo2rWk+wqE5Eg6TArCM="
	mockNodeAddressPrefix = "fdae:41e4:649b:9303:2a07:9c7:5b08:aef7/64"
)

func (srv mockServer) Provision(ctx context.Context, req *pb.ProvisionRequest) (*pb.ProvisionResponse, error) {
	return &pb.ProvisionResponse{
		ServerEndpoint:    mockServerEndpoint,
		ServerAddress:     mockServerAddress,
		ServerPublicKey:   mockServerPublicKey,
		NodeAddressPrefix: mockNodeAddressPrefix,
	}, nil
}

func (suite *ManagerSuite) TestReconcile() {
	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), networkStatus))

	nodeAddress := netip.MustParsePrefix(mockNodeAddressPrefix)

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		addressResource, err := ctest.Get[*network.AddressSpec](
			suite,
			resource.NewMetadata(
				network.ConfigNamespaceName,
				network.AddressSpecType,
				network.LayeredID(
					network.ConfigOperator,
					network.AddressID(constants.SideroLinkName, nodeAddress),
				),
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		address := addressResource.TypedSpec()

		suite.Assert().Equal(nodeAddress, address.Address)
		suite.Assert().Equal(network.ConfigOperator, address.ConfigLayer)
		suite.Assert().Equal(nethelpers.FamilyInet6, address.Family)
		suite.Assert().Equal(constants.SideroLinkName, address.LinkName)

		linkResource, err := ctest.Get[*network.LinkSpec](
			suite,
			resource.NewMetadata(
				network.ConfigNamespaceName,
				network.LinkSpecType,
				network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)),
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		link := linkResource.TypedSpec()

		suite.Assert().Equal("wireguard", link.Kind)
		suite.Assert().Equal(network.ConfigOperator, link.ConfigLayer)
		suite.Assert().NotEmpty(link.Wireguard.PrivateKey)
		suite.Assert().Len(link.Wireguard.Peers, 1)
		suite.Assert().Equal(mockServerEndpoint, link.Wireguard.Peers[0].Endpoint)
		suite.Assert().Equal(mockServerPublicKey, link.Wireguard.Peers[0].PublicKey)
		suite.Assert().Equal(
			[]netip.Prefix{
				netip.PrefixFrom(
					netip.MustParseAddr(mockServerAddress),
					128,
				),
			}, link.Wireguard.Peers[0].AllowedIPs,
		)
		suite.Assert().Equal(
			constants.SideroLinkDefaultPeerKeepalive,
			link.Wireguard.Peers[0].PersistentKeepaliveInterval,
		)

		return nil
	})
}

func TestParseJoinToken(t *testing.T) {
	t.Run("parses a join token from a complete URL without error", func(t *testing.T) {
		// when
		endpoint, err := siderolinkctrl.ParseAPIEndpoint("grpc://10.5.0.2:3445?jointoken=ttt")

		// then
		assert.NoError(t, err)
		assert.Equal(t, siderolinkctrl.APIEndpoint{
			Host:      "10.5.0.2:3445",
			Insecure:  true,
			JoinToken: pointer.To("ttt"),
		}, endpoint)
	})

	t.Run("parses a join token from a secure URL without error", func(t *testing.T) {
		// when
		endpoint, err := siderolinkctrl.ParseAPIEndpoint("https://10.5.0.2:3445?jointoken=ttt&jointoken=xxx")

		// then
		assert.NoError(t, err)
		assert.Equal(t, siderolinkctrl.APIEndpoint{
			Host:      "10.5.0.2:3445",
			Insecure:  false,
			JoinToken: pointer.To("ttt"),
		}, endpoint)
	})

	t.Run("parses a join token from a secure URL without port", func(t *testing.T) {
		// when
		endpoint, err := siderolinkctrl.ParseAPIEndpoint("https://10.5.0.2?jointoken=ttt&jointoken=xxx")

		// then
		assert.NoError(t, err)
		assert.Equal(t, siderolinkctrl.APIEndpoint{
			Host:      "10.5.0.2:443",
			Insecure:  false,
			JoinToken: pointer.To("ttt"),
		}, endpoint)
	})

	t.Run("parses a join token from an URL without a scheme", func(t *testing.T) {
		// when
		endpoint, err := siderolinkctrl.ParseAPIEndpoint("10.5.0.2:3445?jointoken=ttt")

		// then
		assert.NoError(t, err)
		assert.Equal(t, siderolinkctrl.APIEndpoint{
			Host:      "10.5.0.2:3445",
			Insecure:  true,
			JoinToken: pointer.To("ttt"),
		}, endpoint)
	})

	t.Run("does not error if there is no join token in a complete URL", func(t *testing.T) {
		// when
		endpoint, err := siderolinkctrl.ParseAPIEndpoint("grpc://10.5.0.2:3445")

		// then
		assert.NoError(t, err)
		assert.Equal(t, siderolinkctrl.APIEndpoint{
			Host:      "10.5.0.2:3445",
			Insecure:  true,
			JoinToken: nil,
		}, endpoint)
	})

	t.Run("does not error if there is no join token in an URL without a scheme", func(t *testing.T) {
		// when
		endpoint, err := siderolinkctrl.ParseAPIEndpoint("10.5.0.2:3445")

		// then
		assert.NoError(t, err)
		assert.Equal(t, siderolinkctrl.APIEndpoint{
			Host:      "10.5.0.2:3445",
			Insecure:  true,
			JoinToken: nil,
		}, endpoint)
	})
}
