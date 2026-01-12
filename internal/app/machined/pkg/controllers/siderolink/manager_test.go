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

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/stretchr/testify/assert"
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
	t.Parallel()

	if fipsmode.Strict() {
		t.Skip("skipping test in strict FIPS mode")
	}

	suite.Run(t, &ManagerSuite{})
}

type ManagerSuite struct {
	ctest.DefaultSuite

	s *grpc.Server
}

type mockServer struct {
	pb.UnimplementedProvisionServiceServer

	suite     *ManagerSuite
	endpoints []string
}

const (
	mockNodeUUID          = "71233efd-7a07-43f8-b6ba-da90fae0e88b"
	mockUniqueToken       = "random-token"
	mockServerEndpoint1   = "127.0.0.11:51820"
	mockServerEndpoint2   = "localhost:51821"
	mockServerAddress     = "fdae:41e4:649b:9303:b6db:d99c:215e:dfc4"
	mockServerPublicKey   = "2aq/V91QyrHAoH24RK0bldukgo2rWk+wqE5Eg6TArCM="
	mockNodeAddressPrefix = "fdae:41e4:649b:9303:2a07:9c7:5b08:aef7/64"
)

func (srv mockServer) Provision(_ context.Context, req *pb.ProvisionRequest) (*pb.ProvisionResponse, error) {
	srv.suite.Assert().Equal(mockNodeUUID, req.GetNodeUuid())
	srv.suite.Assert().Empty(req.GetJoinToken())
	srv.suite.Assert().False(req.GetWireguardOverGrpc())
	srv.suite.Assert().Equal(mockUniqueToken, req.GetNodeUniqueToken())

	return &pb.ProvisionResponse{
		ServerEndpoint:    pb.MakeEndpoints(srv.endpoints...),
		ServerAddress:     mockServerAddress,
		ServerPublicKey:   mockServerPublicKey,
		NodeAddressPrefix: mockNodeAddressPrefix,
	}, nil
}

func (suite *ManagerSuite) initialSetup(endpoints ...string) {
	lis, err := (&net.ListenConfig{}).Listen(suite.Ctx(), "tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.s = grpc.NewServer()
	pb.RegisterProvisionServiceServer(suite.s, mockServer{
		suite:     suite,
		endpoints: endpoints,
	})

	suite.T().Cleanup(suite.s.Stop)

	go func() {
		suite.Require().NoError(suite.s.Serve(lis))
	}()

	cmdline := procfs.NewCmdline(fmt.Sprintf("%s=%s", constants.KernelParamSideroLink, lis.Addr().String()))
	configController := siderolinkctrl.ConfigController{Cmdline: cmdline}

	suite.Require().NoError(suite.Runtime().RegisterController(&siderolinkctrl.ManagerController{}))
	suite.Require().NoError(suite.Runtime().RegisterController(&configController))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	suite.Create(networkStatus)

	systemInformation := hardware.NewSystemInformation(hardware.SystemInformationID)
	systemInformation.TypedSpec().UUID = mockNodeUUID
	suite.Create(systemInformation)

	uniqToken := runtime.NewUniqueMachineToken()
	uniqToken.TypedSpec().Token = mockUniqueToken
	suite.Create(uniqToken)
}

func (suite *ManagerSuite) TestReconcile() {
	suite.initialSetup(mockServerEndpoint1)

	nodeAddress := netip.MustParsePrefix(mockNodeAddressPrefix)

	ctest.AssertResource(suite,
		network.LayeredID(network.ConfigOperator, network.AddressID(constants.SideroLinkName, nodeAddress)),
		func(r *network.AddressSpec, asrt *assert.Assertions) {
			address := r.TypedSpec()

			asrt.Equal(nodeAddress, address.Address)
			asrt.Equal(network.ConfigOperator, address.ConfigLayer)
			asrt.Equal(nethelpers.FamilyInet6, address.Family)
			asrt.Equal(constants.SideroLinkName, address.LinkName)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.AssertResource(suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)),
		func(r *network.LinkSpec, asrt *assert.Assertions) {
			link := r.TypedSpec()

			asrt.Equal("wireguard", link.Kind)
			asrt.Equal(network.ConfigOperator, link.ConfigLayer)
			asrt.NotEmpty(link.Wireguard.PrivateKey)
			asrt.Len(link.Wireguard.Peers, 1)
			asrt.Equal(mockServerEndpoint1, link.Wireguard.Peers[0].Endpoint)
			asrt.Equal(mockServerPublicKey, link.Wireguard.Peers[0].PublicKey)
			asrt.Equal(
				[]netip.Prefix{
					netip.PrefixFrom(
						netip.MustParseAddr(mockServerAddress),
						128,
					),
				}, link.Wireguard.Peers[0].AllowedIPs,
			)
			asrt.Equal(
				constants.SideroLinkDefaultPeerKeepalive,
				link.Wireguard.Peers[0].PersistentKeepaliveInterval,
			)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	// remove config
	configPtr := siderolink.NewConfig(config.NamespaceName, siderolink.ConfigID).Metadata()
	destroyErr := suite.State().Destroy(suite.Ctx(), configPtr,
		state.WithDestroyOwner(pointer.To(siderolinkctrl.ConfigController{}).Name()))
	suite.Require().NoError(destroyErr)

	ctest.AssertNoResource[*network.LinkSpec](suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)),
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)

	ctest.AssertNoResource[*network.AddressSpec](suite,
		network.LayeredID(network.ConfigOperator, network.AddressID(constants.SideroLinkName, nodeAddress)),
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *ManagerSuite) TestMultipleEndpoints() {
	suite.initialSetup(mockServerEndpoint1, mockServerEndpoint2)

	ctest.AssertResource(suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)),
		func(r *network.LinkSpec, asrt *assert.Assertions) {
			link := r.TypedSpec()

			asrt.Len(link.Wireguard.Peers, 1)
			// Talos should pick the first endpoint from the list.
			asrt.Equal(mockServerEndpoint1, link.Wireguard.Peers[0].Endpoint)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}

func (suite *ManagerSuite) TestResolveEndpoints() {
	suite.initialSetup(mockServerEndpoint2)

	ctest.AssertResource(suite,
		network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)),
		func(r *network.LinkSpec, asrt *assert.Assertions) {
			link := r.TypedSpec()

			asrt.Len(link.Wireguard.Peers, 1)
			// Talos should resolve the hostname to an IP address.
			asrt.Equal("127.0.0.1:51821", link.Wireguard.Peers[0].Endpoint)
		},
		rtestutils.WithNamespace(network.ConfigNamespaceName),
	)
}
