// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dialer_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
)

// startConnectProxy runs a minimal HTTP CONNECT proxy which tunnels every request to backend
// (ignoring the requested destination) and reports the requested destination via the channel.
func startConnectProxy(t *testing.T, backend string) (string, <-chan string) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() { lis.Close() }) //nolint:errcheck

	requested := make(chan string, 16)

	go func() {
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close() //nolint:errcheck

				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil || req.Method != http.MethodConnect {
					return
				}

				requested <- req.URL.Host

				upstream, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", backend)
				if err != nil {
					return
				}

				defer upstream.Close() //nolint:errcheck

				if _, err = io.WriteString(conn, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
					return
				}

				go io.Copy(upstream, conn) //nolint:errcheck

				io.Copy(conn, upstream) //nolint:errcheck
			}()
		}
	}()

	return lis.Addr().String(), requested
}

func startGRPCServer(t *testing.T) string {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()

	go srv.Serve(lis) //nolint:errcheck

	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

// invoke performs a call against a server with no services registered, so a successful tunnel yields Unimplemented.
func invoke(t *testing.T, endpoint string) {
	t.Helper()

	proxy := dialer.NewDynamicProxy(nil)

	conn, err := grpc.NewClient(
		proxy.Target(endpoint),
		append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, proxy.DialOptions()...)...,
	)
	require.NoError(t, err)

	t.Cleanup(func() { conn.Close() }) //nolint:errcheck

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	err = conn.Invoke(ctx, "/talos.test.Service/Method", &emptypb.Empty{}, &emptypb.Empty{})
	require.Equal(t, codes.Unimplemented, status.Code(err), "unexpected error: %v", err)
}

func TestDynamicProxyTarget_HostnamePassedToProxy(t *testing.T) {
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "NO_PROXY", "no_proxy", "https_proxy"} {
		t.Setenv(name, "")
	}

	backend := startGRPCServer(t)
	proxyAddr, requested := startConnectProxy(t, backend)

	t.Setenv("HTTPS_PROXY", "http://"+proxyAddr)

	// the hostname is never resolved: the proxy gets it as is and tunnels to the backend
	_, port, err := net.SplitHostPort(backend)
	require.NoError(t, err)

	endpoint := net.JoinHostPort("discovery.talos.dev", port)

	invoke(t, endpoint)

	select {
	case got := <-requested:
		assert.Equal(t, endpoint, got)
	case <-time.After(10 * time.Second):
		t.Fatal("proxy didn't receive a CONNECT request")
	}
}

func TestDynamicProxyTarget_Direct(t *testing.T) {
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(name, "")
	}

	backend := startGRPCServer(t)

	invoke(t, backend)
}
