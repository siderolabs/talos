// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	ntpclient "github.com/beevik/ntp"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/talos/internal/pkg/ntp"
	timeapi "github.com/siderolabs/talos/pkg/machinery/api/time"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ConfigProvider defines an interface sufficient for the TimeServer.
type ConfigProvider interface {
	Config() config.Config
}

// TimeServer implements TimeService API.
type TimeServer struct {
	timeapi.UnimplementedTimeServiceServer

	ConfigProvider ConfigProvider
	State          state.State
}

// Register implements the factory.Registrator interface.
func (r *TimeServer) Register(s *grpc.Server) {
	timeapi.RegisterTimeServiceServer(s, r)
}

// Time issues a query to the configured ntp server and displays the results.
func (r *TimeServer) Time(ctx context.Context, in *emptypb.Empty) (reply *timeapi.TimeResponse, err error) {
	var timeServer string

	// prefer the node's runtime NTP server list, which includes servers
	// provided via DHCP (the static time.servers config is a subset of it)
	if r.State != nil {
		timeServersStatus, err := safe.ReaderGet[*network.TimeServerStatus](
			ctx,
			r.State,
			resource.NewMetadata(network.NamespaceName, network.TimeServerStatusType, network.TimeServerID, resource.VersionUndefined),
		)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return nil, fmt.Errorf("error getting time server status: %w", err)
			}
		} else if timeServers := timeServersStatus.TypedSpec().NTPServers; len(timeServers) > 0 {
			timeServer = timeServers[0]
		}
	}

	// fall back to the static time.servers config
	if timeServer == "" {
		if r.ConfigProvider.Config() != nil {
			if cfg := r.ConfigProvider.Config().NetworkTimeSyncConfig(); cfg != nil {
				if servers := cfg.Servers(); len(servers) > 0 {
					timeServer = servers[0]
				}
			}
		}
	}

	if timeServer == "" {
		timeServer = constants.DefaultNTPServer
	}

	return r.TimeCheck(ctx, &timeapi.TimeRequest{
		Server: timeServer,
	})
}

// TimeCheck issues a query to the specified ntp server and displays the results.
func (r *TimeServer) TimeCheck(ctx context.Context, in *timeapi.TimeRequest) (reply *timeapi.TimeResponse, err error) {
	var t time.Time

	if in.GetServer() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "ntp server is required")
	}

	if ntp.IsPTPDevice(in.Server) {
		ts, err := ntp.QueryPTPDevice(in.Server)
		if err != nil {
			return nil, fmt.Errorf("error querying PTP device %q: %w", in.Server, err)
		}

		t = time.Unix(ts.Sec, ts.Nsec)
	} else {
		rt, err := ntpclient.Query(in.Server)
		if err != nil {
			return nil, fmt.Errorf("error querying NTP server %q: %w", in.Server, err)
		}

		if err = rt.Validate(); err != nil {
			return nil, fmt.Errorf("error validating NTP response: %w", err)
		}

		t = rt.Time
	}

	return &timeapi.TimeResponse{
		Messages: []*timeapi.Time{
			{
				Server:     in.Server,
				Localtime:  timestamppb.New(time.Now()),
				Remotetime: timestamppb.New(t),
			},
		},
	}, nil
}
