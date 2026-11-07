// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package proxy implements a DNS upstream proxy over plain DNS (UDP/TCP) and DNS-over-TLS.
//
// The proxy keeps idle TCP and TLS connections to the upstream for reuse, uses a fresh UDP socket
// (and so a random source port) and a random query ID for every upstream query, and runs health
// checks on demand.
package proxy

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/miekg/dns"

	"github.com/siderolabs/talos/internal/pkg/dns/internal/probe"
)

const (
	// dialTimeout caps the time spent establishing a connection (including the TLS handshake) to the upstream.
	dialTimeout = 4 * time.Second
	// readTimeout and writeTimeout cap the time waiting for the upstream for a single query.
	readTimeout  = 2 * time.Second
	writeTimeout = 2 * time.Second

	// idleTimeout is the time after which an unused TCP/TLS connection is closed.
	idleTimeout = 10 * time.Second
	// maxIdleConns is the maximum number of idle connections kept per transport.
	maxIdleConns = 32

	// healthcheckTimeout caps a single health check query.
	healthcheckTimeout = time.Second
)

// Network types used by the proxy.
const (
	netUDP = "udp"
	netTCP = "tcp"
	netTLS = "tcp-tls"
)

// ErrClosed is returned when the proxy is used after it was closed.
var ErrClosed = errors.New("proxy is closed")

// Proxy is a plain DNS or DNS-over-TLS upstream.
type Proxy struct {
	tlsConfig *tls.Config

	stop chan struct{}

	idle   map[string][]*idleConn
	addr   string
	probe  probe.Probe
	wg     sync.WaitGroup
	mu     sync.Mutex
	fails  atomic.Uint32
	closed bool
	start  sync.Once
}

type idleConn struct {
	conn *dns.Conn
	used time.Time
}

// New creates a new proxy to the upstream at addr ("host:port").
//
// If tlsConfig is not nil, the queries are sent over DNS-over-TLS, otherwise over plain DNS
// using the protocol the query was received over (UDP or TCP).
func New(addr string, tlsConfig *tls.Config) *Proxy {
	return &Proxy{
		addr:      addr,
		tlsConfig: tlsConfig,
		idle:      map[string][]*idleConn{},
		stop:      make(chan struct{}),
	}
}

// Addr returns the upstream address.
func (p *Proxy) Addr() string { return p.addr }

// Fails returns the number of failed health checks since the last successful one.
func (p *Proxy) Fails() uint32 { return p.fails.Load() }

// Start sets the interval between health check probes, and starts closing idle connections in the background.
func (p *Proxy) Start(interval time.Duration) {
	p.probe.Start(interval)

	p.start.Do(func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if p.closed {
			return
		}

		p.wg.Go(p.cleanupLoop)
	})
}

// Close closes all connections and stops the health checks.
func (p *Proxy) Close() {
	p.probe.Stop()

	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()

		return
	}

	p.closed = true
	close(p.stop)

	idle := p.idle
	p.idle = nil

	p.mu.Unlock()

	for _, conns := range idle {
		for _, ic := range conns {
			ic.conn.Close() //nolint:errcheck
		}
	}

	p.wg.Wait()
}

// Exchange sends the query to the upstream and returns the response.
//
// The proto is the protocol ("udp" or "tcp") to use for plain DNS, it's ignored for DNS-over-TLS.
// If the UDP response is truncated, the query is retried over TCP.
func (p *Proxy) Exchange(ctx context.Context, req *dns.Msg, proto string) (*dns.Msg, error) {
	switch {
	case p.tlsConfig != nil:
		proto = netTLS
	case proto != netTCP:
		proto = netUDP
	}

	// use a random ID for each upstream query, so that the response can't be spoofed by guessing the client's ID
	query := *req
	query.Id = dns.Id()

	wire, err := query.Pack()
	if err != nil {
		return nil, err
	}

	resp, err := p.exchangeRetry(ctx, proto, wire, query.Id, udpSize(req))
	if err != nil {
		return nil, err
	}

	if resp.Truncated && proto == netUDP {
		// the response doesn't fit into UDP, retry over TCP
		resp, err = p.exchangeRetry(ctx, netTCP, wire, query.Id, 0)
		if err != nil {
			return nil, err
		}
	}

	resp.Id = req.Id

	return resp, nil
}

// exchangeRetry sends the query, retrying if an idle connection was closed by the upstream.
func (p *Proxy) exchangeRetry(ctx context.Context, proto string, wire []byte, id uint16, udpSize uint16) (*dns.Msg, error) {
	for {
		resp, cached, err := p.exchange(ctx, proto, wire, id, udpSize)
		if err != nil {
			if cached && isClosedByPeer(err) {
				// an idle connection was closed by the upstream, retry with another one
				continue
			}

			return nil, err
		}

		return resp, nil
	}
}

func (p *Proxy) exchange(ctx context.Context, proto string, wire []byte, id uint16, udpSize uint16) (*dns.Msg, bool, error) {
	conn, cached, err := p.conn(ctx, proto)
	if err != nil {
		return nil, false, err
	}

	conn.UDPSize = udpSize

	if err = write(ctx, conn, wire); err != nil {
		conn.Close() //nolint:errcheck

		return nil, cached, err
	}

	resp, err := read(ctx, conn, id, proto == netUDP)
	if err != nil {
		conn.Close() //nolint:errcheck

		return nil, cached, err
	}

	p.release(proto, conn)

	return resp, false, nil
}

func write(ctx context.Context, conn *dns.Conn, wire []byte) error {
	if err := conn.SetWriteDeadline(deadline(ctx, writeTimeout)); err != nil {
		return err
	}

	_, err := conn.Write(wire)

	return err
}

// read reads the response with the expected ID.
//
// If the UDP response doesn't fit into the buffer, it's returned as a truncated response.
func read(ctx context.Context, conn *dns.Conn, id uint16, isUDP bool) (*dns.Msg, error) {
	if err := conn.SetReadDeadline(deadline(ctx, readTimeout)); err != nil {
		return nil, err
	}

	for {
		resp, err := conn.ReadMsg()
		if err == nil {
			if resp.Id == id {
				return resp, nil
			}

			// a response to a different query, keep reading
			continue
		}

		if !isUDP {
			return nil, err
		}

		if isStrayPacket(resp, err, id) {
			continue
		}

		if resp != nil && isOverflow(err) {
			return truncated(resp), nil
		}

		return nil, err
	}
}

// isStrayPacket returns true if the UDP read error is caused by a packet which is not the response to the query.
func isStrayPacket(resp *dns.Msg, err error, id uint16) bool {
	return (resp == nil && errors.Is(err, dns.ErrShortRead)) || (resp != nil && resp.Id != id)
}

// conn returns an idle connection to the upstream, or dials a new one.
//
// UDP sockets are never reused: a fresh socket per query randomizes the source port,
// which makes spoofing the response harder.
func (p *Proxy) conn(ctx context.Context, proto string) (*dns.Conn, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}

	conn, err := p.idleConn(proto)
	if err != nil {
		return nil, false, err
	}

	if conn != nil {
		return conn, true, nil
	}

	client := dns.Client{
		Net:       proto,
		Dialer:    &net.Dialer{Timeout: dialTimeout, Control: recvErr},
		TLSConfig: p.tlsConfig,
	}

	conn, err = client.DialContext(ctx, p.addr)
	if err != nil {
		return nil, false, err
	}

	return conn, false, nil
}

// idleConn returns an idle connection from the pool, or nil if there is none.
//
// Expired connections found on the way are closed (outside of the lock, as closing
// a TLS connection sends close_notify and might block).
func (p *Proxy) idleConn(proto string) (*dns.Conn, error) {
	var (
		expired []*idleConn
		conn    *dns.Conn
	)

	p.mu.Lock()

	if p.closed {
		p.mu.Unlock()

		return nil, ErrClosed
	}

	// the most recently used connection is the least likely to be closed by the upstream (UDP is never pooled)
	for conns := p.idle[proto]; len(conns) > 0; conns = p.idle[proto] {
		ic := conns[len(conns)-1]
		p.idle[proto] = conns[:len(conns)-1]

		if time.Since(ic.used) > idleTimeout {
			expired = append(expired, ic)

			continue
		}

		conn = ic.conn

		break
	}

	p.mu.Unlock()

	for _, ic := range expired {
		ic.conn.Close() //nolint:errcheck
	}

	return conn, nil
}

// release returns the connection to the idle pool, or closes it if it can't be reused.
func (p *Proxy) release(proto string, conn *dns.Conn) {
	if proto == netUDP {
		conn.Close() //nolint:errcheck

		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed || len(p.idle[proto]) >= maxIdleConns {
		conn.Close() //nolint:errcheck

		return
	}

	p.idle[proto] = append(p.idle[proto], &idleConn{conn: conn, used: time.Now()})
}

func (p *Proxy) cleanupLoop() {
	ticker := time.NewTicker(idleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

// cleanup closes the connections which were idle for too long.
func (p *Proxy) cleanup() {
	var expired []*idleConn

	p.mu.Lock()

	staleTime := time.Now().Add(-idleTimeout)

	for proto, conns := range p.idle {
		alive := conns[:0]

		for _, ic := range conns {
			if ic.used.Before(staleTime) {
				expired = append(expired, ic)
			} else {
				alive = append(alive, ic)
			}
		}

		clear(conns[len(alive):])
		p.idle[proto] = alive
	}

	p.mu.Unlock()

	for _, ic := range expired {
		ic.conn.Close() //nolint:errcheck
	}
}

// Healthcheck kicks off a round of health checks for this proxy.
//
// The health check sends a ". IN NS" query to the upstream every probe interval
// until it succeeds. If a health check is already in progress, this is a no-op.
func (p *Proxy) Healthcheck() {
	p.probe.Do(func() error {
		if err := p.healthcheck(); err != nil {
			if p.fails.Load() < ^uint32(0) {
				p.fails.Add(1)
			}

			return err
		}

		p.fails.Store(0)

		return nil
	})
}

func (p *Proxy) healthcheck() error {
	client := dns.Client{
		Net:          netUDP,
		Dialer:       &net.Dialer{Timeout: 2 * healthcheckTimeout, Control: recvErr},
		ReadTimeout:  healthcheckTimeout,
		WriteTimeout: healthcheckTimeout,
	}

	if p.tlsConfig != nil {
		client.Net = netTLS
		client.TLSConfig = p.tlsConfig
	}

	ping := new(dns.Msg)
	ping.SetQuestion(".", dns.TypeNS)

	resp, _, err := client.Exchange(ping, p.addr)
	if err != nil && resp != nil && (resp.Response || resp.Opcode == dns.OpcodeQuery) {
		// any response from the upstream means it's alive
		err = nil
	}

	return err
}

// udpSize returns the UDP buffer size for the upstream response based on the client's request.
func udpSize(req *dns.Msg) uint16 {
	if opt := req.IsEdns0(); opt != nil {
		return max(opt.UDPSize(), dns.MinMsgSize)
	}

	return dns.MinMsgSize
}

func deadline(ctx context.Context, timeout time.Duration) time.Time {
	d := time.Now().Add(timeout)

	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(d) {
		return ctxDeadline
	}

	return d
}

// isClosedByPeer returns true if the error indicates that the connection was closed by the upstream.
func isClosedByPeer(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}

// isOverflow returns true if the error indicates that the response didn't fit into the buffer.
func isOverflow(err error) bool {
	var dnsErr *dns.Error

	return (errors.As(err, &dnsErr) && errors.Is(err, dns.ErrBuf)) || strings.Contains(err.Error(), "overflow")
}

func truncated(resp *dns.Msg) *dns.Msg {
	resp.Answer = nil
	resp.Ns = nil
	resp.Extra = nil
	resp.Truncated = true

	return resp
}
