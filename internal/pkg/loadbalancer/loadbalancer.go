// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package loadbalancer provides simple TCP loadbalancer.
package loadbalancer

import (
	"context"
	"log"
	"net"

	"inet.af/tcpproxy"

	"github.com/talos-systems/talos/internal/pkg/loadbalancer/upstream"
)

// TCP is a simple loadbalancer for TCP connections across a set of upstreams.
//
// Healthcheck is defined as TCP dial attempt by default.
//
// Zero value of TCP is a valid proxy, use `AddRoute` to install load balancer for
// address.
//
// Usage: call Run() to start lb and wait for shutdown, call Close() to shutdown lb.
type TCP struct {
	tcpproxy.Proxy
}

type lbUpstream string

func (upstream lbUpstream) HealthCheck(ctx context.Context) error {
	d := net.Dialer{}

	c, err := d.DialContext(ctx, "tcp", string(upstream))
	if err != nil {
		log.Printf("healthcheck failed for %q: %s", string(upstream), err)

		return err
	}

	return c.Close()
}

type lbTarget struct {
	list *upstream.List
}

func (target *lbTarget) HandleConn(conn net.Conn) {
	upstreamBackend, err := target.list.Pick()
	if err != nil {
		log.Printf("no upstreams available, closing connection from %s", conn.RemoteAddr())
		conn.Close() //nolint: errcheck

		return
	}

	upstreamAddr := upstreamBackend.(lbUpstream) //nolint: errcheck

	log.Printf("proxying connection %s -> %s", conn.RemoteAddr(), string(upstreamAddr))

	upstreamTarget := tcpproxy.To(string(upstreamAddr))
	upstreamTarget.OnDialError = func(src net.Conn, dstDialErr error) {
		src.Close() //nolint: errcheck

		log.Printf("error dialing upstream %s: %s", string(upstreamAddr), dstDialErr)

		target.list.Down(upstreamBackend)
	}

	upstreamTarget.HandleConn(conn)
}

// AddRoute installs load balancer route from listen address ipAddr to list of upstreams.
//
// TCP automatically does background health checks for the upstreams and picks only healthy
// ones. Healthcheck is simple Dial attempt.
func (t *TCP) AddRoute(ipPort string, upstreamAddrs []string, options ...upstream.ListOption) error {
	upstreams := make([]upstream.Backend, len(upstreamAddrs))
	for i := range upstreams {
		upstreams[i] = lbUpstream(upstreamAddrs[i])
	}

	list, err := upstream.NewList(upstreams, options...)
	if err != nil {
		return err
	}

	t.Proxy.AddRoute(ipPort, &lbTarget{list: list})

	return nil
}
