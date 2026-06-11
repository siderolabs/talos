// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// ProxyURLSuite verifies that proxy-url in a talosconfig context routes
// talosctl→API connections through the specified proxy.
type ProxyURLSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ProxyURLSuite) SuiteName() string {
	return "api.ProxyURLSuite"
}

// SetupTest ...
func (suite *ProxyURLSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *ProxyURLSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestProxyURL verifies that setting proxy-url on a talosconfig context causes
// outgoing API connections to be routed through the proxy.
func (suite *ProxyURLSuite) TestProxyURL() {
	var connectCount atomic.Int32

	ln, err := (&net.ListenConfig{}).Listen(suite.ctx, "tcp", "127.0.0.1:0")
	suite.Require().NoError(err)

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodConnect {
				http.Error(w, "only CONNECT supported", http.StatusMethodNotAllowed)

				return
			}

			connectCount.Add(1)

			dialCtx, dialCancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer dialCancel()

			dst, dialErr := (&net.Dialer{}).DialContext(dialCtx, "tcp", r.Host)
			if dialErr != nil {
				http.Error(w, dialErr.Error(), http.StatusServiceUnavailable)

				return
			}

			hijacker, ok := w.(http.Hijacker)
			if !ok {
				dst.Close() //nolint:errcheck

				http.Error(w, "hijacking not supported", http.StatusInternalServerError)

				return
			}

			src, _, hijackErr := hijacker.Hijack()
			if hijackErr != nil {
				dst.Close() //nolint:errcheck

				http.Error(w, hijackErr.Error(), http.StatusServiceUnavailable)

				return
			}

			src.Write([]byte("HTTP/1.0 200 Connection established\r\n\r\n")) //nolint:errcheck

			done := make(chan struct{}, 2)

			go func() { io.Copy(dst, src); done <- struct{}{} }() //nolint:errcheck
			go func() { io.Copy(src, dst); done <- struct{}{} }() //nolint:errcheck

			<-done

			dst.Close() //nolint:errcheck
			src.Close() //nolint:errcheck
		}),
	}

	go srv.Serve(ln) //nolint:errcheck

	defer srv.Close() //nolint:errcheck

	proxyAddr := "http://" + ln.Addr().String()

	currentCtx := suite.Talosconfig.Contexts[suite.Talosconfig.Context]
	suite.Require().NotNil(currentCtx)

	ctxWithProxy := *currentCtx
	ctxWithProxy.ProxyURL = proxyAddr

	node := suite.RandomDiscoveredNodeInternalIP()

	// The proxy runs on the test runner, so it must be able to reach the gRPC
	// endpoint it tunnels to. In cloud environments the node internal IP is not
	// routable from the runner, so use the suite-configured endpoint (or the
	// talosconfig context endpoints) which is reachable, and keep the node
	// internal IP only for the per-node routing header below.
	endpoint := suite.Endpoint
	if endpoint == "" {
		suite.Require().NotEmpty(currentCtx.Endpoints, "no endpoints configured for the current context")

		endpoint = currentCtx.Endpoints[0]
	}

	c, err := client.New(suite.ctx,
		client.WithConfigContext(&ctxWithProxy),
		client.WithEndpoints(endpoint),
	)
	suite.Require().NoError(err)

	defer c.Close() //nolint:errcheck

	nodeCtx := client.WithNode(suite.ctx, node)

	_, err = c.Version(nodeCtx)
	suite.Require().NoError(err)

	suite.Assert().Positive(connectCount.Load(), "expected at least one CONNECT tunneling request through the proxy")
}

func init() {
	allSuites = append(allSuites, new(ProxyURLSuite))
}
