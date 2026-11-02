// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// hostDialer dials like a host-network container does.
func hostDialer(ctx context.Context, address string) (net.Conn, error) {
	var dialer net.Dialer

	return dialer.DialContext(ctx, "tcp", address)
}

// listenerPort returns the loopback port a listener is bound to.
func listenerPort(t *testing.T, address string) uint32 {
	t.Helper()

	_, portStr, err := net.SplitHostPort(address)
	require.NoError(t, err)

	port, err := strconv.ParseUint(portStr, 10, 16)
	require.NoError(t, err)

	return uint32(port)
}

// closedPort returns a loopback port nothing listens on.
func closedPort(t *testing.T) uint32 {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	port := listenerPort(t, listener.Addr().String())

	require.NoError(t, listener.Close())

	return port
}

func runProbe(t *testing.T, healthCheck containers.ContainerHealthCheckSpec) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	return containersctrl.NewProbeWithDialer(healthCheck, hostDialer)(ctx)
}

func TestProbeTCP(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { listener.Close() }) //nolint:errcheck

	assert.NoError(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind: string(config.ContainerHealthCheckKindTCP),
		Port: listenerPort(t, listener.Addr().String()),
	}))

	assert.Error(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind: string(config.ContainerHealthCheckKindTCP),
		Port: closedPort(t),
	}))
}

func TestProbeHTTP(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/moved":
			// Not followed: a redirect is a pass on its own.
			http.Redirect(w, r, "http://192.0.2.1/", http.StatusFound)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))

	t.Cleanup(server.Close)

	port := listenerPort(t, server.Listener.Addr().String())

	for _, test := range []struct {
		path    string
		healthy bool
	}{
		{path: "/healthz", healthy: true},
		{path: "/moved", healthy: true},
		{path: "/broken", healthy: false},
	} {
		err := runProbe(t, containers.ContainerHealthCheckSpec{
			Kind: string(config.ContainerHealthCheckKindHTTP),
			Port: port,
			Path: test.path,
		})

		if test.healthy {
			assert.NoError(t, err, test.path)
		} else {
			assert.ErrorContains(t, err, "503", test.path)
		}
	}

	assert.Error(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind: string(config.ContainerHealthCheckKindHTTP),
		Port: closedPort(t),
		Path: "/",
	}))
}

func TestProbeGRPC(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("api", healthpb.HealthCheckResponse_NOT_SERVING)

	server := grpc.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)

	go server.Serve(listener) //nolint:errcheck

	t.Cleanup(server.Stop)

	port := listenerPort(t, listener.Addr().String())

	assert.NoError(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind: string(config.ContainerHealthCheckKindGRPC),
		Port: port,
	}))

	assert.ErrorContains(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind:        string(config.ContainerHealthCheckKindGRPC),
		Port:        port,
		GRPCService: "api",
	}), "NOT_SERVING")

	assert.Error(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind:        string(config.ContainerHealthCheckKindGRPC),
		Port:        port,
		GRPCService: "unknown",
	}))
}

// TestProbeLoopThreshold covers the failure threshold: a failure below it reports nothing, so a
// container that never passed stays undecided and one that passed stays healthy.
func TestProbeLoopThreshold(t *testing.T) {
	t.Parallel()

	type outcome struct {
		healthy bool
		message string
	}

	// Each check returns the next result in turn; the loop is stopped once they run out.
	results := []error{
		assert.AnError, // 1 failure, below the threshold: nothing reported
		nil,            // pass
		assert.AnError, // 1 failure after a pass: still healthy, nothing reported
		assert.AnError, // 2 consecutive failures: unhealthy
		assert.AnError, // still unhealthy, reported again
		nil,            // pass again
	}

	var (
		mu       sync.Mutex
		outcomes []outcome
		calls    int
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	probe := func(context.Context) error {
		mu.Lock()
		defer mu.Unlock()

		if calls == len(results) {
			cancel()

			return nil
		}

		err := results[calls]
		calls++

		return err
	}

	containersctrl.RunProbeLoop(ctx, containers.ContainerHealthCheckSpec{
		Interval:         time.Millisecond,
		Timeout:          time.Second,
		FailureThreshold: 2,
	}, true, probe, func(healthy bool, message string) {
		mu.Lock()
		defer mu.Unlock()

		outcomes = append(outcomes, outcome{healthy: healthy, message: message})
	})

	assert.Equal(t, []outcome{
		{healthy: true},
		{healthy: false, message: assert.AnError.Error()},
		{healthy: false, message: assert.AnError.Error()},
		{healthy: true},
	}, outcomes)
}

// TestProbeLoopIntervalAfterCheck covers a check that runs up to its timeout: the next one still
// waits a full interval after it, rather than starting right behind it.
func TestProbeLoopIntervalAfterCheck(t *testing.T) {
	t.Parallel()

	const (
		interval = 50 * time.Millisecond
		window   = 500 * time.Millisecond
	)

	var (
		mu    sync.Mutex
		calls int
	)

	ctx, cancel := context.WithTimeout(t.Context(), window)
	defer cancel()

	// Hangs until the check times out, which is as long as the interval.
	probe := func(checkCtx context.Context) error {
		mu.Lock()
		calls++
		mu.Unlock()

		<-checkCtx.Done()

		return checkCtx.Err()
	}

	containersctrl.RunProbeLoop(ctx, containers.ContainerHealthCheckSpec{
		Interval:         interval,
		Timeout:          interval,
		FailureThreshold: 1,
	}, true, probe, func(bool, string) {})

	mu.Lock()
	defer mu.Unlock()

	// Back-to-back checks would fit window/interval = 10; one interval between them fits about 5.
	assert.LessOrEqual(t, calls, 6, "checks started back to back")
}

// TestProbeLoopInvalidSchedule covers a zero interval, which the config layer never produces: the
// loop reports it instead of spinning.
func TestProbeLoopInvalidSchedule(t *testing.T) {
	t.Parallel()

	var (
		healthy  bool
		message  string
		reported bool
	)

	containersctrl.RunProbeLoop(t.Context(), containers.ContainerHealthCheckSpec{Timeout: time.Second}, true,
		func(context.Context) error {
			t.Fatal("probe must not run")

			return nil
		},
		func(h bool, m string) {
			healthy, message, reported = h, m, true
		})

	assert.True(t, reported)
	assert.False(t, healthy)
	assert.Contains(t, message, "invalid health check schedule")
}

func TestProbeUnsupportedKind(t *testing.T) {
	t.Parallel()

	assert.EqualError(t, runProbe(t, containers.ContainerHealthCheckSpec{Kind: "udp", Port: 80}), `unsupported health check kind "udp"`)
}

// TestProbeHTTPInvalidPath covers a path that cannot be part of a request URL: the check fails
// rather than the probe.
func TestProbeHTTPInvalidPath(t *testing.T) {
	t.Parallel()

	assert.ErrorContains(t, runProbe(t, containers.ContainerHealthCheckSpec{
		Kind: string(config.ContainerHealthCheckKindHTTP),
		Port: 80,
		Path: "/\x7f",
	}), "invalid control character in URL")
}

// TestNewProbeHostNetwork covers a host-network container: the probe dials from the host network
// namespace, with nothing to release.
func TestNewProbeHostNetwork(t *testing.T) {
	t.Parallel()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { listener.Close() }) //nolint:errcheck

	probe, closer, err := containersctrl.NewProbe(containersctrl.ProbeTarget{
		// Not looked at: the PID only identifies the network namespace of a non-host container.
		PID:         math.MaxUint32,
		HostNetwork: true,
		HealthCheck: containers.ContainerHealthCheckSpec{
			Kind: string(config.ContainerHealthCheckKindTCP),
			Port: listenerPort(t, listener.Addr().String()),
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	assert.NoError(t, probe(ctx))
	assert.NoError(t, closer.Close())
}

// TestNewProbeMissingNetNS covers a task which is already gone: its network namespace cannot be
// opened.
func TestNewProbeMissingNetNS(t *testing.T) {
	t.Parallel()

	_, _, err := containersctrl.NewProbe(containersctrl.ProbeTarget{
		RuntimePID: uint32(os.Getpid()), //nolint:gosec // PIDs fit in 32 bits.
		// Above the kernel's PID limit, so never a live process.
		PID: math.MaxUint32,
		HealthCheck: containers.ContainerHealthCheckSpec{
			Kind: string(config.ContainerHealthCheckKindTCP),
			Port: 80,
		},
	})
	assert.ErrorContains(t, err, "failed to open network namespace of PID 4294967295")
}

// TestNewProbeNetNS covers the default network mode: the probe dials from within the network
// namespace of the task. The test's own namespace stands in for the container's; entering it still
// takes CAP_SYS_ADMIN.
func TestNewProbeNetNS(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("entering a network namespace requires root")
	}

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { listener.Close() }) //nolint:errcheck

	probe, closer, err := containersctrl.NewProbe(containersctrl.ProbeTarget{
		// Itself as the runtime: its /proc is the one the PID belongs to.
		RuntimePID: uint32(os.Getpid()), //nolint:gosec // PIDs fit in 32 bits.
		PID:        uint32(os.Getpid()), //nolint:gosec // PIDs fit in 32 bits.
		HealthCheck: containers.ContainerHealthCheckSpec{
			Kind: string(config.ContainerHealthCheckKindTCP),
			Port: listenerPort(t, listener.Addr().String()),
		},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	assert.NoError(t, probe(ctx))
	assert.NoError(t, closer.Close())
}

// TestNetNSPath covers resolving the task PID through the runtime's procfs: with workload isolation
// the task PID belongs to the sandbox PID namespace, which machined's own /proc does not show.
func TestNetNSPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/proc/7/root/proc/42/ns/net", containersctrl.NetNSPath(7, 42))
}
