// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/beevik/ntp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	timeapi "github.com/talos-systems/talos/pkg/machinery/api/time"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// ConfigProvider defines an interface sufficient for the TimeServer.
type ConfigProvider interface {
	Config() config.Provider
}

// TimeServer implements TimeService API.
type TimeServer struct {
	timeapi.UnimplementedTimeServiceServer

	ConfigProvider ConfigProvider
}

// Register implements the factory.Registrator interface.
func (r *TimeServer) Register(s *grpc.Server) {
	timeapi.RegisterTimeServiceServer(s, r)
}

// Time issues a query to the configured ntp server and displays the results.
func (r *TimeServer) Time(ctx context.Context, in *emptypb.Empty) (reply *timeapi.TimeResponse, err error) {
	timeServers := r.ConfigProvider.Config().Machine().Time().Servers()

	if len(timeServers) == 0 {
		timeServers = []string{constants.DefaultNTPServer}
	}

	return r.TimeCheck(ctx, &timeapi.TimeRequest{
		Server: timeServers[0],
	})
}

// TimeCheck issues a query to the specified ntp server and displays the results.
func (r *TimeServer) TimeCheck(ctx context.Context, in *timeapi.TimeRequest) (reply *timeapi.TimeResponse, err error) {
	rt, err := ntp.Query(in.Server)
	if err != nil {
		return nil, fmt.Errorf("error querying NTP server %q: %w", in.Server, err)
	}

	if err = rt.Validate(); err != nil {
		return nil, fmt.Errorf("error validating NTP response: %w", err)
	}

	return &timeapi.TimeResponse{
		Messages: []*timeapi.Time{
			{
				Server:     in.Server,
				Localtime:  timestamppb.New(time.Now()),
				Remotetime: timestamppb.New(rt.Time),
			},
		},
	}, nil
}
