// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/server"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	networkapi "github.com/talos-systems/talos/pkg/machinery/api/network"
)

type NetworkSuite struct {
	suite.Suite
}

func TestNetwordSuite(t *testing.T) {
	suite.Run(t, new(NetworkSuite))
}

//nolint:dupl
func (suite *NetworkSuite) TestRoutes() {
	server, listener := suite.fakeNetworkRPC()

	//nolint:errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	//nolint:errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(
		fmt.Sprintf("%s://%s", "unix", listener.Addr().String()),
		grpc.WithInsecure(),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	suite.Assert().NoError(err)

	nClient := networkapi.NewNetworkServiceClient(conn)
	resp, err := nClient.Routes(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Greater(len(resp.Messages[0].Routes), 0)
}

//nolint:dupl
func (suite *NetworkSuite) TestInterfaces() {
	server, listener := suite.fakeNetworkRPC()

	//nolint:errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	//nolint:errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(
		fmt.Sprintf("%s://%s", "unix", listener.Addr().String()),
		grpc.WithInsecure(),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	suite.Assert().NoError(err)

	nClient := networkapi.NewNetworkServiceClient(conn)
	resp, err := nClient.Interfaces(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Greater(len(resp.Messages[0].Interfaces), 0)
}

func (suite *NetworkSuite) fakeNetworkRPC() (*grpc.Server, net.Listener) {
	// Create gRPC server
	api := &server.NetworkServer{}

	server := factory.NewServer(api)
	tmpfile, err := ioutil.TempFile("", "networkd")
	suite.Assert().NoError(err)

	listener, err := factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(tmpfile.Name()),
	)
	suite.Assert().NoError(err)

	return server, listener
}

func (suite *NetworkSuite) TestToCIDR() {
	suite.Assert().Equal(server.ToCIDR(unix.AF_INET, net.ParseIP("192.168.254.0"), 24), "192.168.254.0/24")
	suite.Assert().Equal(server.ToCIDR(unix.AF_INET6, net.ParseIP("2001:db8::"), 16), "2001:db8::/16")
	suite.Assert().Equal(server.ToCIDR(unix.AF_INET, nil, 0), "0.0.0.0/0")
	suite.Assert().Equal(server.ToCIDR(unix.AF_INET6, nil, 0), "::/0")
}
