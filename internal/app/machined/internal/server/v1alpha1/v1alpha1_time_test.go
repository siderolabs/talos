// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"

	runtime "github.com/siderolabs/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/siderolabs/talos/pkg/grpc/factory"
	timeapi "github.com/siderolabs/talos/pkg/machinery/api/time"
	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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

func (provider *mockConfigProvider) Config() config.Config {
	return container.NewV1Alpha1(&v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineTime: &v1alpha1.TimeConfig{
				TimeServers: []string{provider.timeServer},
			},
		},
	})
}

func (suite *TimedSuite) TestTime() {
	testServer := "time.cloudflare.com"

	nClient := suite.newTimeClient(&runtime.TimeServer{
		ConfigProvider: &mockConfigProvider{timeServer: testServer},
	})

	reply, err := nClient.Time(context.Background(), &emptypb.Empty{})
	suite.Require().NoError(err)
	suite.Assert().Equal(reply.Messages[0].Server, testServer)
}

func (suite *TimedSuite) TestTimeUsesRuntimeTimeServers() {
	// fake NTP server so the test doesn't depend on the network
	ntpAddr := fakeNTPServer(suite.T())

	ctx := context.Background()

	st := state.WrapCore(inmem.NewStateWithOptions()(network.NamespaceName))

	timeServersStatus := network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID)
	timeServersStatus.TypedSpec().NTPServers = []string{ntpAddr}

	suite.Require().NoError(st.Create(ctx, timeServersStatus))

	// the static config has a different server; the runtime list (e.g. from DHCP) should win
	nClient := suite.newTimeClient(&runtime.TimeServer{
		ConfigProvider: &mockConfigProvider{timeServer: "time.cloudflare.com"},
		State:          st,
	})

	reply, err := nClient.Time(ctx, &emptypb.Empty{})
	suite.Require().NoError(err)
	suite.Assert().Equal(reply.Messages[0].Server, ntpAddr)
}

func (suite *TimedSuite) TestTimeCheck() {
	testServer := "time.cloudflare.com"

	// Create ntp client with bogus server
	// so we can check that we explicitly check the time of the
	// specified server ( testserver )

	nClient := suite.newTimeClient(&runtime.TimeServer{})

	reply, err := nClient.TimeCheck(context.Background(), &timeapi.TimeRequest{Server: testServer})
	suite.Require().NoError(err)
	suite.Assert().Equal(reply.Messages[0].Server, testServer)
}

func (suite *TimedSuite) newTimeClient(api *runtime.TimeServer) timeapi.TimeServiceClient {
	server := factory.NewServer(api)
	listener, err := fakeTimedRPC(suite.T())
	suite.Assert().NoError(err)

	suite.T().Cleanup(server.Stop)                                    //nolint:errcheck
	suite.T().Cleanup(func() { os.Remove(listener.Addr().String()) }) //nolint:errcheck

	//nolint:errcheck
	go server.Serve(listener)

	conn, err := grpc.NewClient(
		fmt.Sprintf("%s://%s", "unix", listener.Addr().String()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer.DialUnix()),
	)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { conn.Close() }) //nolint:errcheck

	return timeapi.NewTimeServiceClient(conn)
}

// ntpEpochOffset is the number of seconds between the NTP epoch (1900-01-01) and the Unix epoch.
const ntpEpochOffset = 2208988800

// fakeNTPServer starts a minimal UDP NTP responder on loopback and returns its address.
func fakeNTPServer(t *testing.T) string {
	t.Helper()

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { conn.Close() }) //nolint:errcheck

	go func() {
		buf := make([]byte, 512)

		for {
			n, raddr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}

			if n < 48 {
				continue
			}

			resp := make([]byte, 48)

			now := uint64(time.Now().Unix()) + ntpEpochOffset

			resp[0] = 0x24 // leap 0, version 4, mode 4 (server)
			resp[1] = 1    // stratum 1

			// reference time: now - 1s, so the response is fresh
			putNTPTime(resp[16:24], now-1)
			// origin time: echo the request's transmit timestamp
			copy(resp[24:32], buf[40:48])
			// receive and transmit times
			putNTPTime(resp[32:40], now)
			putNTPTime(resp[40:48], now)

			_, err = conn.WriteTo(resp, raddr)
			if err != nil {
				return
			}
		}
	}()

	return conn.LocalAddr().String()
}

// putNTPTime writes a 64-bit NTP timestamp (seconds since the NTP epoch, zero fraction) into b.
func putNTPTime(b []byte, secs uint64) {
	binary.BigEndian.PutUint64(b, secs<<32)
}

func fakeTimedRPC(t *testing.T) (net.Listener, error) {
	t.Helper()

	tmpfile, err := os.CreateTemp(t.TempDir(), "timed")
	require.NoError(t, err)

	return factory.NewListener(
		t.Context(),
		factory.Network("unix"),
		factory.SocketPath(tmpfile.Name()),
	)
}
