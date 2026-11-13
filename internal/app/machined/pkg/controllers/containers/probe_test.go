// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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
		// Not known, and not needed: the PID only identifies the network namespace of a non-host
		// container.
		PID:         0,
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
		// Above the kernel's PID limit, so never a live process.
		PID: math.MaxUint32,
		HealthCheck: containers.ContainerHealthCheckSpec{
			Kind: string(config.ContainerHealthCheckKindTCP),
			Port: 80,
		},
	})
	assert.ErrorContains(t, err, "failed to open PID 4294967295")
}

// TestNewProbeUnknownPID covers a task whose PID the runner could not resolve: without it there is no
// network namespace to connect from, and /proc/0 must not be looked up.
func TestNewProbeUnknownPID(t *testing.T) {
	t.Parallel()

	_, _, err := containersctrl.NewProbe(containersctrl.ProbeTarget{
		HealthCheck: containers.ContainerHealthCheckSpec{
			Kind: string(config.ContainerHealthCheckKindTCP),
			Port: 80,
		},
	})
	assert.ErrorContains(t, err, "container PID is not known")
}

// isolatedNetNS starts an OS thread in a new network namespace, with loopback up, standing in for a
// container's task.
//
// It returns the thread's ID, which procfs resolves like a PID, and a function running code on that
// thread: a socket opened there stays in the namespace wherever it is used from afterwards.
func isolatedNetNS(t *testing.T) (uint32, func(func())) {
	t.Helper()

	type started struct {
		tid uint32
		err error
	}

	startedCh := make(chan started, 1)
	work := make(chan func())
	done := make(chan struct{})

	go func() {
		// Never unlocked: the thread is left in the namespace, so it is thrown away when this
		// goroutine returns rather than handed back to the scheduler.
		runtime.LockOSThread()

		if err := unix.Unshare(unix.CLONE_NEWNET); err != nil {
			startedCh <- started{err: err}

			return
		}

		if err := setLoopbackUp(); err != nil {
			startedCh <- started{err: err}

			return
		}

		startedCh <- started{tid: uint32(unix.Gettid())} //nolint:gosec // Thread IDs fit in 32 bits.

		for {
			select {
			case fn := <-work:
				fn()

				done <- struct{}{}
			case <-t.Context().Done():
				return
			}
		}
	}()

	result := <-startedCh
	require.NoError(t, result.err)

	return result.tid, func(fn func()) {
		work <- fn

		<-done
	}
}

// setLoopbackUp brings up the loopback interface of the calling thread's network namespace, which
// starts out down.
func setLoopbackUp() error {
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	lo, err := net.InterfaceByName("lo")
	if err != nil {
		return err
	}

	return conn.Link.Set(&rtnetlink.LinkMessage{
		Index:  uint32(lo.Index), //nolint:gosec // Interface indexes are positive.
		Flags:  unix.IFF_UP,
		Change: unix.IFF_UP,
	})
}

// TestNewProbeNetNS covers the default network mode: the probe dials from within the network
// namespace of the task, for every kind of check, and nowhere else.
//
// The task is a thread in a namespace of its own, so a probe which dialed from the host namespace
// would not find the servers listening there, and would find the one only listening on the host.
// Creating and entering a network namespace takes CAP_SYS_ADMIN.
func TestNewProbeNetNS(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("creating and entering a network namespace requires root")
	}

	tid, inNetNS := isolatedNetNS(t)

	listen := func() net.Listener {
		var (
			listener net.Listener
			err      error
		)

		inNetNS(func() {
			listener, err = (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		})

		require.NoError(t, err)

		t.Cleanup(func() { listener.Close() }) //nolint:errcheck

		return listener
	}

	tcpListener := listen()

	httpServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	httpServer.Listener = listen()
	httpServer.Start()

	t.Cleanup(httpServer.Close)

	grpcListener := listen()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	grpcServer := grpc.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	go grpcServer.Serve(grpcListener) //nolint:errcheck

	t.Cleanup(grpcServer.Stop)

	// Only on the host: nothing listens on this port in the task's namespace.
	hostListener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { hostListener.Close() }) //nolint:errcheck

	for _, test := range []struct {
		name        string
		healthCheck containers.ContainerHealthCheckSpec
		healthy     bool
	}{
		{
			name: "tcp",
			healthCheck: containers.ContainerHealthCheckSpec{
				Kind: string(config.ContainerHealthCheckKindTCP),
				Port: listenerPort(t, tcpListener.Addr().String()),
			},
			healthy: true,
		},
		{
			name: "http",
			healthCheck: containers.ContainerHealthCheckSpec{
				Kind: string(config.ContainerHealthCheckKindHTTP),
				Port: listenerPort(t, httpServer.Listener.Addr().String()),
				Path: "/healthz",
			},
			healthy: true,
		},
		{
			name: "grpc",
			healthCheck: containers.ContainerHealthCheckSpec{
				Kind: string(config.ContainerHealthCheckKindGRPC),
				Port: listenerPort(t, grpcListener.Addr().String()),
			},
			healthy: true,
		},
		{
			name: "host only",
			healthCheck: containers.ContainerHealthCheckSpec{
				Kind: string(config.ContainerHealthCheckKindTCP),
				Port: listenerPort(t, hostListener.Addr().String()),
			},
			healthy: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Opened directly rather than through NewProbe: a thread ID cannot be pinned with a
			// pidfd, which TestOpenTaskNetNS covers with a process of its own.
			netNS, err := ns.GetNS(containersctrl.NetNSPath(tid))
			require.NoError(t, err)

			t.Cleanup(func() { assert.NoError(t, netNS.Close()) })

			probe := containersctrl.NewProbeWithDialer(test.healthCheck, containersctrl.NetNSDialer(netNS))

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			if test.healthy {
				assert.NoError(t, probe(ctx))
			} else {
				assert.ErrorIs(t, probe(ctx), syscall.ECONNREFUSED)
			}
		})
	}
}

// TestNetNSPath covers the task PID being looked up in machined's own procfs: the runner has already
// resolved it into machined's PID namespace, even with workload isolation.
func TestNetNSPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/proc/42/ns/net", containersctrl.NetNSPath(42))
}

// TestCheckCgroup covers telling the container task apart by the cgroups procfs lists for it.
func TestCheckCgroup(t *testing.T) {
	t.Parallel()

	const cgroupPath = "/taloscontainers/web"

	for _, test := range []struct {
		name     string
		contents string
		wantErr  string
	}{
		{name: "unified", contents: "0::/taloscontainers/web\n"},
		{name: "unified below", contents: "0::/taloscontainers/web/nested\n"},
		{name: "cgroup v1", contents: "12:pids:/system\n4:memory:/taloscontainers/web\n"},
		{
			name:     "other container",
			contents: "0::/taloscontainers/web2\n",
			wantErr:  `it runs in cgroup "/taloscontainers/web2", not "/taloscontainers/web"`,
		},
		{
			name:     "other process",
			contents: "0::/system/runtime\n",
			wantErr:  `it runs in cgroup "/system/runtime", not "/taloscontainers/web"`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "cgroup")
			require.NoError(t, os.WriteFile(path, []byte(test.contents), 0o600))

			err := containersctrl.CheckCgroup(path, cgroupPath)
			if test.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.wantErr)
			}
		})
	}
}

// TestOpenTaskNetNS covers opening the network namespace of a task only while its PID still names a
// process in the container's cgroup: a PID reused by another process after the task exited is
// refused, rather than probed in its stead.
func TestOpenTaskNetNS(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "sleep", "60")
	require.NoError(t, cmd.Start())

	pid := uint32(cmd.Process.Pid) //nolint:gosec // PIDs fit in 32 bits.

	// The child stays in the cgroup of the test.
	contents, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	require.NoError(t, err)

	_, ownCgroup, ok := strings.Cut(strings.TrimSpace(string(contents)), "0::")
	if !ok || strings.Contains(ownCgroup, "\n") {
		t.Skip("the cgroup of the test is only checked on the unified hierarchy")
	}

	netNS, err := containersctrl.OpenTaskNetNS(pid, ownCgroup)
	require.NoError(t, err)
	assert.NoError(t, netNS.Close())

	_, err = containersctrl.OpenTaskNetNS(pid, "/taloscontainers/web")
	assert.ErrorContains(t, err, fmt.Sprintf("PID %d is not the container task", pid))

	require.NoError(t, cmd.Process.Kill())
	assert.Error(t, cmd.Wait())

	// Gone, or, should the PID have been reused already, another process outside the cgroup.
	_, err = containersctrl.OpenTaskNetNS(pid, ownCgroup)
	assert.Error(t, err)
}
