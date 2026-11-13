// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containernetworking/plugins/pkg/ns"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
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
	// PID of the container's task in machined's PID namespace, whose network namespace the probe
	// connects from.
	//
	// The runner already resolves the task PID out of the CRI containerd's PID namespace, which is a
	// sandbox one when workload isolation is enabled. It is 0 if the runner could not resolve it.
	PID uint32
	// CgroupPath is the cgroup the container's task runs in, as /proc/<pid>/cgroup shows it to
	// machined. It tells the task apart from another process which got its PID after it exited.
	CgroupPath string
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
// The container's network namespace is opened here, and held until the returned closer is called:
// every check then runs in that namespace even if the PID is reused by another process before the
// controller notices the task has exited.
func NewProbe(target ProbeTarget) (Probe, io.Closer, error) {
	if target.HostNetwork {
		var dialer net.Dialer

		return NewProbeWithDialer(target.HealthCheck, func(ctx context.Context, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp", address)
		}), nopCloser{}, nil
	}

	netNS, err := openTaskNetNS(target.PID, target.CgroupPath)
	if err != nil {
		return nil, nil, err
	}

	return NewProbeWithDialer(target.HealthCheck, netNSDialer(netNS)), netNS, nil
}

// errPIDUnknown is the probe failure of a container outside the host network whose task PID the
// runner could not resolve: there is no network namespace to connect from.
var errPIDUnknown = errors.New("container PID is not known, its network namespace cannot be probed")

// openTaskNetNS opens the network namespace of the container task pid, which runs in cgroupPath.
//
// By the time it is opened, pid may name another process: the task may have exited and its PID been
// reused. So the process is pinned with a pidfd first, checked to run in the container's cgroup, and
// checked to be still alive once its network namespace is open. While it is alive its PID cannot be
// reused, so the cgroup read and the namespace opened in between are both its own.
func openTaskNetNS(pid uint32, cgroupPath string) (ns.NetNS, error) {
	if pid == 0 {
		return nil, errPIDUnknown
	}

	pidfd, err := unix.PidfdOpen(int(pid), 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open PID %d: %w", pid, err)
	}

	defer unix.Close(pidfd) //nolint:errcheck

	if err = checkCgroup(procCgroupPath(pid), cgroupPath); err != nil {
		return nil, fmt.Errorf("PID %d is not the container task: %w", pid, err)
	}

	netNS, err := ns.GetNS(netNSPath(pid))
	if err != nil {
		return nil, fmt.Errorf("failed to open network namespace of PID %d: %w", pid, err)
	}

	if err = unix.PidfdSendSignal(pidfd, 0, nil, 0); err != nil {
		netNS.Close() //nolint:errcheck

		return nil, fmt.Errorf("PID %d exited while its network namespace was opened: %w", pid, err)
	}

	return netNS, nil
}

// checkCgroup returns an error unless the process whose cgroup membership is listed at procCgroup
// runs in cgroupPath, or in a cgroup below it.
//
// Each line is hierarchy-ID:controllers:path: a single one on the unified hierarchy, and one per
// hierarchy with cgroup v1, where any hierarchy placing the process there will do.
func checkCgroup(procCgroup, cgroupPath string) error {
	contents, err := os.ReadFile(procCgroup)
	if err != nil {
		return err
	}

	var found []string

	for line := range strings.Lines(string(contents)) {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) != 3 {
			continue
		}

		path := fields[2]

		if path == cgroupPath || strings.HasPrefix(path, cgroupPath+"/") {
			return nil
		}

		found = append(found, path)
	}

	return fmt.Errorf("it runs in cgroup %q, not %q", strings.Join(found, ", "), cgroupPath)
}

// taskCgroupPath returns the cgroup the task of a container runs in, as /proc/<pid>/cgroup shows it
// to machined: below the cgroup root of machined when it is nested.
func taskCgroupPath(containerID string) string {
	return filepath.Join("/", cgroup.Path(containerCgroupPath(containerID)))
}

// netNSPath returns the path of the network namespace of pid, a PID in machined's PID namespace.
func netNSPath(pid uint32) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

// procCgroupPath returns the path listing the cgroups of pid, a PID in machined's PID namespace.
func procCgroupPath(pid uint32) string {
	return fmt.Sprintf("/proc/%d/cgroup", pid)
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
