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

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/siderolabs/go-retry/retry"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	siderolinkctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

func TestManagerSuite(t *testing.T) {
	if fipsmode.Strict() {
		t.Skip("skipping test in strict FIPS mode")
	}

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
		configController := siderolinkctrl.ConfigController{Cmdline: cmdline}

		suite.Require().NoError(suite.Runtime().RegisterController(&siderolinkctrl.ManagerController{}))
		suite.Require().NoError(suite.Runtime().RegisterController(&configController))
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

func (srv mockServer) Provision(_ context.Context, _ *pb.ProvisionRequest) (*pb.ProvisionResponse, error) {
	return &pb.ProvisionResponse{
		ServerEndpoint:    pb.MakeEndpoints(mockServerEndpoint),
		ServerAddress:     mockServerAddress,
		ServerPublicKey:   mockServerPublicKey,
		NodeAddressPrefix: mockNodeAddressPrefix,
	}, nil
}

func (suite *ManagerSuite) TestReconcile() {
	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), networkStatus))

	systemInformation := hardware.NewSystemInformation(hardware.SystemInformationID)
	systemInformation.TypedSpec().UUID = "71233efd-7a07-43f8-b6ba-da90fae0e88b"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), systemInformation))

	uniqToken := runtime.NewUniqueMachineToken()
	uniqToken.TypedSpec().Token = "random-token"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), uniqToken))

	nodeAddress := netip.MustParsePrefix(mockNodeAddressPrefix)

	addressSpec := network.NewAddressSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.AddressID(constants.SideroLinkName, nodeAddress)))
	linkSpec := network.NewLinkSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)))

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		addressResource, err := ctest.Get[*network.AddressSpec](suite, addressSpec.Metadata())
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

		linkResource, err := ctest.Get[*network.LinkSpec](suite, linkSpec.Metadata())
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

	// remove config
	configPtr := siderolink.NewConfig(config.NamespaceName, siderolink.ConfigID).Metadata()
	destroyErr := suite.State().Destroy(suite.Ctx(), configPtr,
		state.WithDestroyOwner(pointer.To(siderolinkctrl.ConfigController{}).Name()))
	suite.Require().NoError(destroyErr)

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		_, err := ctest.Get[*network.LinkSpec](suite, linkSpec.Metadata())
		if err == nil {
			return retry.ExpectedErrorf("link resource still exists")
		}

		suite.Assert().Truef(state.IsNotFoundError(err), "unexpected error: %v", err)

		_, err = ctest.Get[*network.AddressSpec](suite, addressSpec.Metadata())
		if err == nil {
			return retry.ExpectedErrorf("address resource still exists")
		}

		suite.Assert().Truef(state.IsNotFoundError(err), "unexpected error: %v", err)

		return nil
	})
}
