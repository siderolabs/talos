// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint: testpackage
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

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	healthapi "github.com/talos-systems/talos/pkg/machinery/api/health"
	networkapi "github.com/talos-systems/talos/pkg/machinery/api/network"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	suite.Run(t, new(NetworkdSuite))
}

// nolint: dupl
func (suite *NetworkdSuite) TestRoutes() {
	_, server, listener := suite.fakeNetworkdRPC()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	// nolint: errcheck
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

// nolint: dupl
func (suite *NetworkdSuite) TestInterfaces() {
	_, server, listener := suite.fakeNetworkdRPC()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	// nolint: errcheck
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

func (suite *NetworkdSuite) fakeNetworkdRPC() (*networkd.Networkd, *grpc.Server, net.Listener) {
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

	return n, server, listener
}

func (suite *NetworkdSuite) TestToCIDR() {
	suite.Assert().Equal(toCIDR(unix.AF_INET, net.ParseIP("192.168.254.0"), 24), "192.168.254.0/24")
	suite.Assert().Equal(toCIDR(unix.AF_INET6, net.ParseIP("2001:db8::"), 16), "2001:db8::/16")
	suite.Assert().Equal(toCIDR(unix.AF_INET, nil, 0), "0.0.0.0/0")
	suite.Assert().Equal(toCIDR(unix.AF_INET6, nil, 0), "::/0")
}

func (suite *NetworkdSuite) TestHealthAPI() {
	nwd, server, listener := suite.fakeNetworkdRPC()

	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	defer server.Stop()

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(
		fmt.Sprintf("%s://%s", "unix", listener.Addr().String()),
		grpc.WithInsecure(),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	suite.Assert().NoError(err)

	// Verify base state
	nClient := healthapi.NewHealthClient(conn)
	hcResp, err := nClient.Check(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	// Can only check against unknown since its not guaranteed that
	// the host the tests will run on will have an arp table populated.
	suite.Assert().NotEqual(healthapi.HealthCheck_UNKNOWN, hcResp.Messages[0].Status)

	rResp, err := nClient.Ready(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Equal(healthapi.ReadyCheck_NOT_READY, rResp.Messages[0].Status)

	// Trigger ready condition
	nwd.SetReady()
	suite.Assert().NoError(err)
	rResp, err = nClient.Ready(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Equal(healthapi.ReadyCheck_READY, rResp.Messages[0].Status)

	// Test watch
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := nClient.Watch(ctx, &healthapi.HealthWatchRequest{IntervalSeconds: 1})
	suite.Require().NoError(err)

	for i := 0; i < 2; i++ {
		hcResp, err = stream.Recv()
		suite.Assert().NoError(err)
		suite.Assert().NotEqual(healthapi.HealthCheck_UNKNOWN, hcResp.Messages[0].Status)
	}

	cancel()

	_, err = stream.Recv()
	suite.Assert().Error(err)
}
