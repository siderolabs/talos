/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"log"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
	"github.com/talos-systems/talos/internal/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	// Hide all our state transition messages
	//log.SetOutput(ioutil.Discard)
	suite.Run(t, new(NetworkdSuite))
}

func (suite *NetworkdSuite) TestGet() {
	api := NewRegistrator(&userdata.UserData{})
	server := factory.NewServer(api)
	listener, err := fakeNetworkdRPC(suite.T())
	suite.Assert().NoError(err)
	defer server.Stop()

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(listener.Addr().String(), grpc.WithInsecure())
	suite.Assert().NoError(err)
	nwdClient := proto.NewNetworkdClient(conn)

	gr := &proto.GetRequest{
		Func: "get",
		App:  "networkd",
		Args: []string{"-i", "lo"},
	}
	resp, err := nwdClient.Get(context.Background(), gr)
	suite.Assert().NoError(err)
	log.Println(resp)

	/*
		return &InitServiceClient{
			InitClient: proto.NewInitClient(conn),
		}, nil
	*/

}

func fakeNetworkdRPC(t *testing.T) (net.Listener, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Maybe potential for a port collision
	// here
	// TODO May need to override this with a tempdir
	// factory.SocketPath(constants.NetworkdSocketPath),
	return factory.NewListener(
		factory.Port(1024+r.Intn(64485)),
		factory.Network("tcp"),
	)
}
