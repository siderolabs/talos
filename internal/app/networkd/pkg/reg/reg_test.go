// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg

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

	networkapi "github.com/talos-systems/talos/api/network"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/pkg/grpc/factory"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NetworkdSuite))
}

// nolint: dupl
func (suite *NetworkdSuite) TestRoutes() {
	server, listener := suite.fakeNetworkdRPC()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(fmt.Sprintf("%s://%s", "unix", listener.Addr().String()), grpc.WithInsecure())
	suite.Assert().NoError(err)

	nClient := networkapi.NewNetworkServiceClient(conn)
	resp, err := nClient.Routes(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Greater(len(resp.Messages[0].Routes), 0)
}

// nolint: dupl
func (suite *NetworkdSuite) TestInterfaces() {
	server, listener := suite.fakeNetworkdRPC()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(fmt.Sprintf("%s://%s", "unix", listener.Addr().String()), grpc.WithInsecure())
	suite.Assert().NoError(err)

	nClient := networkapi.NewNetworkServiceClient(conn)
	resp, err := nClient.Interfaces(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Greater(len(resp.Messages[0].Interfaces), 0)
}

func (suite *NetworkdSuite) fakeNetworkdRPC() (*grpc.Server, net.Listener) {
	// Create networkd instance
	n, err := networkd.New(nil)
	suite.Assert().NoError(err)

	// Create gRPC server
	api := NewRegistrator(n)
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

func (suite *NetworkdSuite) TestToCIDR() {
	suite.Assert().Equal(toCIDR(unix.AF_INET, net.ParseIP("192.168.254.0"), 24), "192.168.254.0/24")
	suite.Assert().Equal(toCIDR(unix.AF_INET6, net.ParseIP("2001:db8::"), 16), "2001:db8::/16")
	suite.Assert().Equal(toCIDR(unix.AF_INET, nil, 0), "0.0.0.0/0")
	suite.Assert().Equal(toCIDR(unix.AF_INET6, nil, 0), "::/0")
}
