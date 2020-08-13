// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reg provides the gRPC network service implementation.
package reg

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	healthapi "github.com/talos-systems/talos/pkg/machinery/api/health"
	networkapi "github.com/talos-systems/talos/pkg/machinery/api/network"
)

// Registrator is the concrete type that implements the factory.Registrator and
// networkapi.NetworkServer interfaces.
type Registrator struct {
	Networkd *networkd.Networkd
	Conn     *rtnetlink.Conn
}

// NewRegistrator builds new Registrator instance.
func NewRegistrator(n *networkd.Networkd) *Registrator {
	nlConn, err := rtnetlink.Dial(nil)
	if err != nil {
		log.Fatal(err)
	}

	return &Registrator{
		Networkd: n,
		Conn:     nlConn,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	networkapi.RegisterNetworkServiceServer(s, r)
	healthapi.RegisterHealthServer(s, r)
}

// Routes returns the hosts routing table.
func (r *Registrator) Routes(ctx context.Context, in *empty.Empty) (reply *networkapi.RoutesResponse, err error) {
	list, err := r.Conn.Route.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get route list: %w", err)
	}

	routes := []*networkapi.Route{}

	for _, rMesg := range list {
		ifaceData, err := r.Conn.Link.Get((rMesg.Attributes.OutIface))
		if err != nil {
			log.Printf("failed to get interface details for interface index %d: %v", rMesg.Attributes.OutIface, err)
			// TODO: Remove once we get this sorted on why there's a
			// failure here
			log.Printf("%+v", rMesg)

			continue
		}

		routes = append(routes, &networkapi.Route{
			Interface:   ifaceData.Attributes.Name,
			Destination: toCIDR(rMesg.Family, rMesg.Attributes.Dst, int(rMesg.DstLength)),
			Gateway:     toCIDR(rMesg.Family, rMesg.Attributes.Gateway, 32),
			Metric:      rMesg.Attributes.Priority,
			Scope:       uint32(rMesg.Scope),
			Source:      toCIDR(rMesg.Family, rMesg.Attributes.Src, int(rMesg.SrcLength)),
			Family:      networkapi.AddressFamily(rMesg.Family),
			Protocol:    networkapi.RouteProtocol(rMesg.Protocol),
			Flags:       rMesg.Flags,
		})
	}

	return &networkapi.RoutesResponse{
		Messages: []*networkapi.Routes{
			{
				Routes: routes,
			},
		},
	}, nil
}

// Interfaces returns the hosts network interfaces and addresses.
func (r *Registrator) Interfaces(ctx context.Context, in *empty.Empty) (reply *networkapi.InterfacesResponse, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return reply, err
	}

	resp := &networkapi.Interfaces{}

	for _, iface := range ifaces {
		ifaceaddrs, err := iface.Addrs()
		if err != nil {
			return reply, err
		}

		addrs := make([]string, 0, len(ifaceaddrs))
		for _, addr := range ifaceaddrs {
			addrs = append(addrs, addr.String())
		}

		ifmsg := &networkapi.Interface{
			Index:        uint32(iface.Index),
			Mtu:          uint32(iface.MTU),
			Name:         iface.Name,
			Hardwareaddr: iface.HardwareAddr.String(),
			Flags:        networkapi.InterfaceFlags(iface.Flags),
			Ipaddress:    addrs,
		}

		resp.Interfaces = append(resp.Interfaces, ifmsg)
	}

	return &networkapi.InterfacesResponse{
		Messages: []*networkapi.Interfaces{
			resp,
		},
	}, nil
}

func toCIDR(family uint8, prefix net.IP, prefixLen int) string {
	netLen := 32

	if family == unix.AF_INET6 {
		netLen = 128
	}

	// Set a friendly readable value instead of "<nil>"
	if prefix == nil {
		switch family {
		case unix.AF_INET6:
			prefix = net.ParseIP("::")
		case unix.AF_INET:
			prefix = net.ParseIP("0.0.0.0")
		}
	}

	ipNet := &net.IPNet{
		IP:   prefix,
		Mask: net.CIDRMask(prefixLen, netLen),
	}

	return ipNet.String()
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
