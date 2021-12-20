// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	runtime "github.com/talos-systems/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	timeapi "github.com/talos-systems/talos/pkg/machinery/api/time"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

type TimedSuite struct {
	suite.Suite
}

func TestTimedSuite(t *testing.T) {
	// Hide all our state transition messages
	// log.SetOutput(ioutil.Discard)
	suite.Run(t, new(TimedSuite))
}

type mockConfigProvider struct {
	timeServer string
}

func (provider *mockConfigProvider) Config() config.Provider {
	return &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineTime: &v1alpha1.TimeConfig{
				TimeServers: []string{provider.timeServer},
			},
		},
	}
}

func (suite *TimedSuite) TestTime() {
	testServer := "time.cloudflare.com"

	// Create gRPC server
	api := &runtime.TimeServer{
		ConfigProvider: &mockConfigProvider{timeServer: testServer},
	}
	server := factory.NewServer(api)
	listener, err := fakeTimedRPC()
	suite.Assert().NoError(err)

	defer server.Stop()

	//nolint:errcheck
	defer os.Remove(listener.Addr().String())

	//nolint:errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(
		fmt.Sprintf("%s://%s", "unix", listener.Addr().String()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	suite.Require().NoError(err)

	nClient := timeapi.NewTimeServiceClient(conn)
	reply, err := nClient.Time(context.Background(), &emptypb.Empty{})
	suite.Require().NoError(err)
	suite.Assert().Equal(reply.Messages[0].Server, testServer)
}

func (suite *TimedSuite) TestTimeCheck() {
	testServer := "time.cloudflare.com"

	// Create ntp client with bogus server
	// so we can check that we explicitly check the time of the
	// specified server ( testserver )

	// Create gRPC server
	api := &runtime.TimeServer{}
	server := factory.NewServer(api)
	listener, err := fakeTimedRPC()
	suite.Assert().NoError(err)

	defer server.Stop()

	//nolint:errcheck
	defer os.Remove(listener.Addr().String())

	//nolint:errcheck
	go server.Serve(listener)

	conn, err := grpc.Dial(
		fmt.Sprintf("%s://%s", "unix", listener.Addr().String()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	suite.Require().NoError(err)

	nClient := timeapi.NewTimeServiceClient(conn)
	reply, err := nClient.TimeCheck(context.Background(), &timeapi.TimeRequest{Server: testServer})
	suite.Require().NoError(err)
	suite.Assert().Equal(reply.Messages[0].Server, testServer)
}

func fakeTimedRPC() (net.Listener, error) {
	tmpfile, err := ioutil.TempFile("", "timed")
	if err != nil {
		return nil, err
	}

	return factory.NewListener(
		factory.Network("unix"),
		factory.SocketPath(tmpfile.Name()),
	)
}
