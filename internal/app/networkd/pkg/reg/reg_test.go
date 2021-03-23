// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package reg

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	healthapi "github.com/talos-systems/talos/pkg/machinery/api/health"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	suite.Run(t, new(NetworkdSuite))
}

func (suite *NetworkdSuite) fakeNetworkdRPC() (*networkd.Networkd, *grpc.Server, net.Listener) {
	// Create networkd instance
	n, err := networkd.New(log.New(os.Stderr, "", log.LstdFlags), nil)
	suite.Assert().NoError(err)

	// Create gRPC server
	api, err := NewRegistrator(n)
	suite.Require().NoError(err)

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

func (suite *NetworkdSuite) TestHealthAPI() {
	nwd, server, listener := suite.fakeNetworkdRPC()

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
