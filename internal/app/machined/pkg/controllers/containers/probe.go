// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/containernetworking/plugins/pkg/ns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// probeHost is the address every health check connects to.
//
// A container in the default network mode has an empty network namespace with only loopback in it,
// so loopback is the one address that is reachable in every mode.
const probeHost = "127.0.0.1"

// ProbeTarget identifies the container execution to probe, and how.
//
// It is comparable: a change to any field means the running probe no longer matches.
type ProbeTarget struct {
	// RuntimePID is the host PID of the containerd which runs the task.
	//
	// The task PID is only meaningful in that containerd's PID namespace, which is not machined's
	// when workload isolation is enabled.
	RuntimePID uint32
	// PID of the container's task, as seen by the containerd running it, whose network namespace the
	// probe connects from.
	PID uint32
	// HostNetwork is true if the container shares the host network namespace.
	HostNetwork bool
	// HealthCheck to run.
	HealthCheck containers.ContainerHealthCheckSpec
}

// Probe runs a single health check, returning nil if it passed.
type Probe func(ctx context.Context) error

// ContextDialer opens a TCP connection to an address.
type ContextDialer func(ctx context.Context, address string) (net.Conn, error)

// NewProbe builds the probe for a target.
//
// The container's network namespace is opened once, here, and held until the returned closer is
// called: every check then runs in that namespace even if the PID is reused by another process
// before the controller notices the task has exited.
func NewProbe(target ProbeTarget) (Probe, io.Closer, error) {
	if target.HostNetwork {
		var dialer net.Dialer

		return NewProbeWithDialer(target.HealthCheck, func(ctx context.Context, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", address)
		}), nopCloser{}, nil
	}

	netNS, err := ns.GetNS(netNSPath(target.RuntimePID, target.PID))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open network namespace of PID %d: %w", target.PID, err)
	}

	return NewProbeWithDialer(target.HealthCheck, netNSDialer(netNS)), netNS, nil
}

// netNSPath returns the path of the network namespace of pid, as seen by the process runtimePID.
//
// It goes through the procfs mounted in runtimePID's mount namespace: with workload isolation that
// is the procfs of the sandbox PID namespace the task PID belongs to, and without it it is the host
// one. machined's own /proc would resolve a sandbox PID to a missing or an unrelated process.
func netNSPath(runtimePID, pid uint32) string {
	return fmt.Sprintf("/proc/%d/root/proc/%d/ns/net", runtimePID, pid)
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// NewProbeWithDialer builds the probe for a health check, connecting through dial.
func NewProbeWithDialer(healthCheck containers.ContainerHealthCheckSpec, dial ContextDialer) Probe {
	address := net.JoinHostPort(probeHost, strconv.FormatUint(uint64(healthCheck.Port), 10))

	switch config.ContainerHealthCheckKind(healthCheck.Kind) {
	case config.ContainerHealthCheckKindTCP:
		return func(ctx context.Context) error {
			return probeTCP(ctx, dial, address)
		}
	case config.ContainerHealthCheckKindHTTP:
		return newHTTPProbe(dial, "http://"+address+healthCheck.Path)
	case config.ContainerHealthCheckKindGRPC:
		return func(ctx context.Context) error {
			return probeGRPC(ctx, dial, address, healthCheck.GRPCService)
		}
	default:
		return func(context.Context) error {
			return fmt.Errorf("unsupported health check kind %q", healthCheck.Kind)
		}
	}
}

// netNSDialer connects from within an already-open network namespace.
//
// The socket is created while the calling thread is in that namespace, and stays bound to it: once
// the dial returns, the connection can be used from any thread.
func netNSDialer(netNS ns.NetNS) ContextDialer {
	return func(ctx context.Context, address string) (net.Conn, error) {
		var (
			dialer net.Dialer
			conn   net.Conn
		)

		if err := netNS.Do(func(ns.NetNS) error {
			var err error

			conn, err = dialer.DialContext(ctx, "tcp", address)

			return err
		}); err != nil {
			return nil, err
		}

		return conn, nil
	}
}

func probeTCP(ctx context.Context, dial ContextDialer, address string) error {
	conn, err := dial(ctx, address)
	if err != nil {
		return err
	}

	return conn.Close()
}

func newHTTPProbe(dial ContextDialer, url string) Probe {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, address string) (net.Conn, error) {
				return dial(ctx, address)
			},
			DisableKeepAlives: true,
		},
		// A redirect already counts as a pass, and following it could leave the container.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		defer resp.Body.Close() //nolint:errcheck

		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096)) //nolint:errcheck

		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
			return fmt.Errorf("unexpected HTTP status %s", resp.Status)
		}

		return nil
	}
}

func probeGRPC(ctx context.Context, dial ContextDialer, address, service string) error {
	// A fresh connection for every check: a long-lived one would have its own reconnect backoff
	// deciding when the container is looked at again.
	conn, err := grpc.NewClient("passthrough:///"+address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dial),
	)
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		return err
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("gRPC health status %s", resp.GetStatus())
	}

	return nil
}
