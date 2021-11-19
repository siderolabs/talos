// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	pb "github.com/talos-systems/siderolink/api/siderolink"
	"google.golang.org/grpc"
	"inet.af/netaddr"

	siderolinkctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/siderolink"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

type ManagerSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc

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

func (suite *ManagerSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.startRuntime()

	lis, err := net.Listen("tcp", "localhost:0")
	suite.Require().NoError(err)

	suite.s = grpc.NewServer()
	pb.RegisterProvisionServiceServer(suite.s, mockServer{})

	go func() {
		suite.Require().NoError(suite.s.Serve(lis))
	}()

	cmdline := procfs.NewCmdline(fmt.Sprintf("%s=%s", constants.KernelParamSideroLink, lis.Addr().String()))

	suite.Require().NoError(suite.runtime.RegisterController(&siderolinkctrl.ManagerController{
		Cmdline: cmdline,
	}))
}

func (suite *ManagerSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *ManagerSuite) TestReconcile() {
	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true

	suite.Require().NoError(suite.state.Create(suite.ctx, networkStatus))

	nodeAddress := netaddr.MustParseIPPrefix(mockNodeAddressPrefix)

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			addressResource, err := suite.state.Get(suite.ctx, resource.NewMetadata(
				network.ConfigNamespaceName,
				network.AddressSpecType,
				network.LayeredID(network.ConfigOperator, network.AddressID(constants.SideroLinkName, nodeAddress)),
				resource.VersionUndefined,
			))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			address := addressResource.(*network.AddressSpec).TypedSpec()

			suite.Assert().Equal(nodeAddress, address.Address)
			suite.Assert().Equal(network.ConfigOperator, address.ConfigLayer)
			suite.Assert().Equal(nethelpers.FamilyInet6, address.Family)
			suite.Assert().Equal(constants.SideroLinkName, address.LinkName)

			linkResource, err := suite.state.Get(suite.ctx, resource.NewMetadata(
				network.ConfigNamespaceName,
				network.LinkSpecType,
				network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)),
				resource.VersionUndefined,
			))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			link := linkResource.(*network.LinkSpec).TypedSpec()

			suite.Assert().Equal("wireguard", link.Kind)
			suite.Assert().Equal(network.ConfigOperator, link.ConfigLayer)
			suite.Assert().NotEmpty(link.Wireguard.PrivateKey)
			suite.Assert().Len(link.Wireguard.Peers, 1)
			suite.Assert().Equal(mockServerEndpoint, link.Wireguard.Peers[0].Endpoint)
			suite.Assert().Equal(mockServerPublicKey, link.Wireguard.Peers[0].PublicKey)
			suite.Assert().Equal([]netaddr.IPPrefix{netaddr.IPPrefixFrom(netaddr.MustParseIP(mockServerAddress), 128)}, link.Wireguard.Peers[0].AllowedIPs)
			suite.Assert().Equal(constants.SideroLinkDefaultPeerKeepalive, link.Wireguard.Peers[0].PersistentKeepaliveInterval)

			return nil
		}))
}

func (suite *ManagerSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.s.Stop()

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestManagerSuite(t *testing.T) {
	suite.Run(t, new(ManagerSuite))
}
