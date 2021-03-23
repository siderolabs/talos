// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reg provides the gRPC network service implementation.
package reg

import (
	"context"
	"errors"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	healthapi "github.com/talos-systems/talos/pkg/machinery/api/health"
)

// Registrator is the concrete type that implements the factory.Registrator and
// healthapi.HealthServer and networkapi.NetworkServiceServer interfaces.
type Registrator struct {
	healthapi.UnimplementedHealthServer

	Networkd *networkd.Networkd
}

// NewRegistrator builds new Registrator instance.
func NewRegistrator(n *networkd.Networkd) (*Registrator, error) {
	return &Registrator{
		Networkd: n,
	}, nil
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	healthapi.RegisterHealthServer(s, r)
}

// Check implements the Health api and provides visibilty into the state of networkd.
func (r *Registrator) Check(ctx context.Context, in *empty.Empty) (reply *healthapi.HealthCheckResponse, err error) {
	reply = &healthapi.HealthCheckResponse{
		Messages: []*healthapi.HealthCheck{
			{
				Status: healthapi.HealthCheck_SERVING,
			},
		},
	}

	return reply, nil
}

// Watch implements the Health api and provides visibilty into the state of networkd.
// Ready signifies the daemon (api) is healthy and ready to serve requests.
func (r *Registrator) Watch(in *healthapi.HealthWatchRequest, srv healthapi.Health_WatchServer) (err error) {
	if in == nil {
		return errors.New("an input interval is required")
	}

	var (
		resp   *healthapi.HealthCheckResponse
		ticker = time.NewTicker(time.Duration(in.IntervalSeconds) * time.Second)
	)

	defer ticker.Stop()

	for {
		select {
		case <-srv.Context().Done():
			return srv.Context().Err()
		case <-ticker.C:
			resp, err = r.Check(srv.Context(), &empty.Empty{})
			if err != nil {
				return err
			}

			if err = srv.Send(resp); err != nil {
				return err
			}
		}
	}
}

// Ready implements the Health api and provides visibility to the state of networkd.
// Ready signifies the initial network configuration ( interfaces, routes, hostname, resolv.conf )
// settings have been applied.
// Not Ready signifies that the initial network configuration still needs to happen.
func (r *Registrator) Ready(ctx context.Context, in *empty.Empty) (reply *healthapi.ReadyCheckResponse, err error) {
	rdy := &healthapi.ReadyCheck{Status: healthapi.ReadyCheck_NOT_READY}

	if r.Networkd.Ready() {
		rdy.Status = healthapi.ReadyCheck_READY
	}

	reply = &healthapi.ReadyCheckResponse{
		Messages: []*healthapi.ReadyCheck{
			rdy,
		},
	}

	return reply, nil
}
