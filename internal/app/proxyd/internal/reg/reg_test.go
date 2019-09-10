/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

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
	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
	"github.com/talos-systems/talos/internal/app/proxyd/proto"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"google.golang.org/grpc"
)

type ProxydSuite struct {
	suite.Suite
}

func TestProxydSuite(t *testing.T) {
	// Hide all our state transition messages
	log.SetOutput(ioutil.Discard)
	suite.Run(t, new(ProxydSuite))
}

func (suite *ProxydSuite) TestBackends() {
	testBackend := "127.0.0.1"
	// Create reverse proxy
	_, cancel := context.WithCancel(context.Background())
	r, err := frontend.NewReverseProxy([]string{}, cancel)
	suite.Assert().NoError(err)

	r.AddBackend("bootstrap-1", testBackend)
	defer r.Shutdown()

	// Create gRPC server
	api := NewRegistrator(r)
	server := factory.NewServer(api)
	listener, err := fakeProxydRPC()
	suite.Assert().NoError(err)

	defer server.Stop()
	// nolint: errcheck
	defer os.Remove(listener.Addr().String())

	// nolint: errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(fmt.Sprintf("%s://%s", "unix", listener.Addr().String()), grpc.WithInsecure())
	suite.Assert().NoError(err)
	pClient := proto.NewProxydClient(conn)

	resp, err := pClient.Backends(context.Background(), &empty.Empty{})
	suite.Assert().NoError(err)
	suite.Assert().Equal(resp.Backends[0].Addr, testBackend)
	log.Println(resp)
}

func fakeProxydRPC() (net.Listener, error) {
	tmpfile, err := ioutil.TempFile("", "proxyd")
	if err != nil {
		return nil, err
	}

	return factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(tmpfile.Name()),
	)
}
