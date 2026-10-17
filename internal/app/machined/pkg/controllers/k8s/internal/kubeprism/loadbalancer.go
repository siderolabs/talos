// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeprism

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/siderolabs/go-loadbalancer/upstream"
	"github.com/siderolabs/tcpproxy"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Four failures move a fully healthy upstream from the configured high score (3) below zero.
	drainFailureThreshold = 4
)

var healthCheckTierThresholds = []time.Duration{
	0,
	time.Millisecond / 10,
	time.Millisecond,
	10 * time.Millisecond,
	100 * time.Millisecond,
	time.Second,
}

// LoadBalancer routes KubePrism connections to API servers which pass an HTTPS readiness check.
// It falls back to TCP-healthy upstreams only when no upstream passes the readiness check.
type LoadBalancer struct {
	proxy       tcpproxy.Proxy
	logger      *zap.Logger
	health      *healthChecker
	connections *connectionTracker

	l7Upstreams  *upstream.List[backend]
	tcpUpstreams *upstream.List[tcpBackend]

	endpoint        string
	dialTimeout     time.Duration
	keepAlivePeriod time.Duration
	tcpUserTimeout  time.Duration
	healthCheckOpts []upstream.ListOption

	lifecycleMu sync.Mutex
	closing     bool
	connWg      sync.WaitGroup
}

// NewLoadBalancer constructs a KubePrism load balancer.
func NewLoadBalancer(
	endpoint string,
	logger *zap.Logger,
	dialTimeout, keepAlivePeriod, tcpUserTimeout time.Duration,
	healthCheckOptions ...upstream.ListOption,
) *LoadBalancer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &LoadBalancer{
		endpoint:        endpoint,
		logger:          logger.Named("kubeprism-lb"),
		health:          &healthChecker{logger: logger.Named("kubeprism-healthcheck")},
		connections:     newConnectionTracker(),
		dialTimeout:     dialTimeout,
		keepAlivePeriod: keepAlivePeriod,
		tcpUserTimeout:  tcpUserTimeout,
		healthCheckOpts: healthCheckOptions,
	}
}

// SetKubeconfig configures TLS trust and the optional client certificate used by readiness checks.
func (lb *LoadBalancer) SetKubeconfig(data string) error {
	return lb.health.SetKubeconfig(data)
}

// Start starts the listener and begins checking the supplied upstreams.
func (lb *LoadBalancer) Start(upstreams []string) error {
	if lb.l7Upstreams != nil {
		return fmt.Errorf("KubePrism load balancer is already started")
	}

	if err := lb.reconcile(upstreams); err != nil {
		return err
	}

	lb.proxy.AddRoute(lb.endpoint, lb)

	if err := lb.proxy.Start(); err != nil {
		lb.l7Upstreams.Shutdown()
		lb.tcpUpstreams.Shutdown()
		lb.l7Upstreams = nil
		lb.tcpUpstreams = nil

		return err
	}

	return nil
}

// Reconcile updates the upstream set without restarting the listener.
func (lb *LoadBalancer) Reconcile(upstreams []string) error {
	if lb.l7Upstreams == nil {
		return fmt.Errorf("KubePrism load balancer is not started")
	}

	return lb.reconcile(upstreams)
}

func (lb *LoadBalancer) reconcile(addresses []string) error {
	l7Backends := make([]backend, 0, len(addresses))
	tcpBackends := make([]tcpBackend, 0, len(addresses))

	for _, address := range addresses {
		l7Backends = append(l7Backends, backend{address: address, health: lb.health, connections: lb.connections})
		tcpBackends = append(tcpBackends, tcpBackend{address: address})
	}

	if lb.l7Upstreams == nil {
		options := append([]upstream.ListOption(nil), lb.healthCheckOpts...)
		options = append(
			options,
			upstream.WithTiers(0, upstream.Tier(len(healthCheckTierThresholds)-1), 1),
			upstream.WithInitialScore(-1),
		)

		var err error

		lb.l7Upstreams, err = upstream.NewList[backend](sliceSeq(l7Backends), options...)
		if err != nil {
			return fmt.Errorf("failed to create KubePrism readiness upstream list: %w", err)
		}

		tcpOptions := append([]upstream.ListOption(nil), lb.healthCheckOpts...)
		tcpOptions = append(tcpOptions, upstream.WithInitialScore(-1))

		lb.tcpUpstreams, err = upstream.NewList[tcpBackend](sliceSeq(tcpBackends), tcpOptions...)
		if err != nil {
			lb.l7Upstreams.Shutdown()
			lb.l7Upstreams = nil

			return fmt.Errorf("failed to create KubePrism TCP fallback upstream list: %w", err)
		}

		return nil
	}

	lb.l7Upstreams.Reconcile(sliceSeq(l7Backends))
	lb.tcpUpstreams.Reconcile(sliceSeq(tcpBackends))

	return nil
}

// Healthy reports whether at least one upstream is available, including TCP fallback upstreams.
func (lb *LoadBalancer) Healthy() (bool, error) {
	if lb.l7Upstreams == nil || lb.tcpUpstreams == nil {
		return false, fmt.Errorf("KubePrism load balancer is not started")
	}

	if _, err := lb.l7Upstreams.Pick(); err == nil {
		return true, nil
	} else if err != upstream.ErrNoUpstreams {
		return false, err
	}

	if _, err := lb.tcpUpstreams.Pick(); err == nil {
		return true, nil
	} else if err != upstream.ErrNoUpstreams {
		return false, err
	}

	return false, nil
}

// Shutdown closes the listener, active proxied connections, and health check loops.
func (lb *LoadBalancer) Shutdown() error {
	if lb.l7Upstreams == nil {
		return nil
	}

	lb.lifecycleMu.Lock()
	lb.closing = true
	lb.lifecycleMu.Unlock()

	if err := lb.proxy.Close(); err != nil {
		return err
	}

	_ = lb.proxy.Wait()
	lb.connections.closeAll()
	lb.connWg.Wait()
	lb.l7Upstreams.Shutdown()
	lb.tcpUpstreams.Shutdown()
	lb.l7Upstreams = nil
	lb.tcpUpstreams = nil

	return nil
}

// HandleConn implements tcpproxy.Target.
func (lb *LoadBalancer) HandleConn(src net.Conn) {
	lb.lifecycleMu.Lock()
	if lb.closing {
		lb.lifecycleMu.Unlock()
		_ = src.Close()

		return
	}

	lb.connWg.Add(1)
	lb.lifecycleMu.Unlock()
	defer lb.connWg.Done()

	selected, err := lb.l7Upstreams.Pick()
	useReadinessCheck := err == nil

	if useReadinessCheck {
		err = lb.proxyConnection(src, selected.address)
	} else if errors.Is(err, upstream.ErrNoUpstreams) {
		var tcpSelected tcpBackend

		tcpSelected, err = lb.tcpUpstreams.Pick()
		if err == nil {
			err = lb.proxyConnection(src, tcpSelected.address)
		}
	} else {
		_ = src.Close()
	}

	if err != nil {
		lb.logger.Warn("failed to proxy connection", zap.Error(err), zap.String("remote_addr", src.RemoteAddr().String()))

		var dialErr upstreamDialError
		if errors.As(err, &dialErr) {
			lb.l7Upstreams.Down(backend{address: dialErr.address, health: lb.health, connections: lb.connections})
			lb.tcpUpstreams.Down(tcpBackend{address: dialErr.address})
			lb.connections.drain(dialErr.address)
		} else if errors.Is(err, upstream.ErrNoUpstreams) {
			_ = src.Close()
		}
	}
}

func (lb *LoadBalancer) proxyConnection(src net.Conn, address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), lb.dialTimeout)
	defer cancel()

	dst, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	if err != nil {
		_ = src.Close()

		return upstreamDialError{address: address, err: err}
	}

	if lb.keepAlivePeriod > 0 {
		for _, conn := range []net.Conn{tcpproxy.UnderlyingConn(src), dst} {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				_ = tcpConn.SetKeepAlive(true)
				_ = tcpConn.SetKeepAlivePeriod(lb.keepAlivePeriod)
			}
		}
	}

	if lb.tcpUserTimeout > 0 {
		tcpproxy.SetTCPUserTimeout(src, lb.tcpUserTimeout)
		tcpproxy.SetTCPUserTimeout(dst, lb.tcpUserTimeout)
	}

	tracked := lb.connections.add(address, src, dst)
	if tracked == nil {
		return nil
	}

	defer lb.connections.remove(address, tracked)

	errCh := make(chan error, 2)

	go copyConn(errCh, dst, src)
	go copyConn(errCh, src, dst)

	firstErr := <-errCh
	if firstErr != nil {
		tracked.close()
	}

	secondErr := <-errCh
	if firstErr != nil && !errors.Is(firstErr, net.ErrClosed) {
		lb.logger.Debug("connection relay ended", zap.String("upstream_addr", address), zap.Error(firstErr))
	}
	if secondErr != nil && !errors.Is(secondErr, net.ErrClosed) {
		lb.logger.Debug("connection relay ended", zap.String("upstream_addr", address), zap.Error(secondErr))
	}

	return nil
}

type upstreamDialError struct {
	address string
	err     error
}

func (err upstreamDialError) Error() string {
	return fmt.Sprintf("failed to dial upstream %q: %v", err.address, err.err)
}

func (err upstreamDialError) Unwrap() error {
	return err.err
}

func copyConn(errCh chan<- error, dst, src net.Conn) {
	dst = tcpproxy.UnderlyingConn(dst)
	src = tcpproxy.UnderlyingConn(src)

	_, err := io.Copy(dst, src)

	if tcpConn, ok := dst.(*net.TCPConn); ok {
		_ = tcpConn.CloseWrite()
	}

	if tcpConn, ok := src.(*net.TCPConn); ok {
		_ = tcpConn.CloseRead()
	}

	errCh <- err
}

type backend struct {
	address     string
	health      *healthChecker
	connections *connectionTracker
}

func (backend backend) HealthCheck(ctx context.Context) (upstream.Tier, error) {
	start := time.Now()

	err := backend.health.check(ctx, backend.address)
	if err != nil {
		if backend.connections.recordFailure(backend.address) {
			backend.connections.drain(backend.address)
		}

		backend.health.logger.Warn("KubePrism upstream readiness check failed",
			zap.String("address", backend.address),
			zap.Error(err),
		)

		return -1, err
	}

	backend.connections.recordSuccess(backend.address)

	elapsed := time.Since(start)
	for tier := len(healthCheckTierThresholds) - 1; tier >= 0; tier-- {
		if elapsed >= healthCheckTierThresholds[tier] {
			return upstream.Tier(tier), nil
		}
	}

	return 0, nil
}

type tcpBackend struct {
	address string
}

func (backend tcpBackend) HealthCheck(ctx context.Context) (upstream.Tier, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", backend.address)
	if err != nil {
		return -1, err
	}

	return 0, conn.Close()
}

type healthChecker struct {
	logger *zap.Logger
	mu     sync.RWMutex
	tls    *tls.Config
}

func (checker *healthChecker) SetKubeconfig(data string) error {
	if data == "" {
		checker.mu.Lock()
		checker.tls = nil
		checker.mu.Unlock()

		return nil
	}

	kubeconfig, err := clientcmd.Load([]byte(data))
	if err != nil {
		return fmt.Errorf("failed to parse KubePrism health check kubeconfig: %w", err)
	}

	currentContext := kubeconfig.Contexts[kubeconfig.CurrentContext]
	if currentContext == nil {
		return fmt.Errorf("KubePrism health check kubeconfig has no current context")
	}

	cluster := kubeconfig.Clusters[currentContext.Cluster]
	if cluster == nil {
		return fmt.Errorf("KubePrism health check kubeconfig has no current cluster")
	}

	if len(cluster.CertificateAuthorityData) == 0 {
		return fmt.Errorf("KubePrism health check kubeconfig has no embedded cluster CA")
	}

	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(cluster.CertificateAuthorityData) {
		return fmt.Errorf("KubePrism health check kubeconfig has invalid cluster CA certificates")
	}

	var certificates []tls.Certificate

	authInfo := kubeconfig.AuthInfos[currentContext.AuthInfo]
	if authInfo == nil {
		return fmt.Errorf("KubePrism health check kubeconfig has no current auth info")
	}

	if len(authInfo.ClientCertificateData) != 0 || len(authInfo.ClientKeyData) != 0 {
		if len(authInfo.ClientCertificateData) == 0 || len(authInfo.ClientKeyData) == 0 {
			return fmt.Errorf("KubePrism health check kubeconfig has an incomplete client certificate")
		}

		certificate, certErr := tls.X509KeyPair(authInfo.ClientCertificateData, authInfo.ClientKeyData)
		if certErr != nil {
			return fmt.Errorf("failed to parse KubePrism health check client certificate: %w", certErr)
		}

		certificates = append(certificates, certificate)
	}

	checker.mu.Lock()
	checker.tls = &tls.Config{
		RootCAs:      rootCAs,
		Certificates: certificates,
		MinVersion:   tls.VersionTLS12,
	}
	checker.mu.Unlock()

	return nil
}

func (checker *healthChecker) check(ctx context.Context, address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid KubePrism upstream address %q: %w", address, err)
	}

	checker.mu.RLock()
	tlsConfig := checker.tls
	if tlsConfig != nil {
		tlsConfig = tlsConfig.Clone()
		tlsConfig.ServerName = host
	}
	checker.mu.RUnlock()

	if tlsConfig == nil {
		return fmt.Errorf("KubePrism upstream %q cannot be checked before the cluster CA is available", address)
	}

	checkURL := (&url.URL{
		Scheme: "https",
		Host:   address,
		Path:   "/readyz",
	}).String()

	statusCode, requestErr := requestReadyz(ctx, checkURL, tlsConfig, false)
	if requestErr == nil && statusCode == http.StatusOK {
		return nil
	}

	if requestErr == nil && statusCode != http.StatusUnauthorized && statusCode != http.StatusForbidden {
		return fmt.Errorf("KubePrism upstream %q returned HTTP status %d from /readyz", address, statusCode)
	}

	if len(tlsConfig.Certificates) == 0 {
		if requestErr != nil {
			return requestErr
		}

		return fmt.Errorf("KubePrism upstream %q returned HTTP status %d from /readyz", address, statusCode)
	}

	// Anonymous access to /readyz is preferred. Retry with the node-local admin
	// certificate for configurations which require client certificate authentication.
	tlsConfig.Certificates = append([]tls.Certificate(nil), tlsConfig.Certificates...)
	statusCode, requestErr = requestReadyz(ctx, checkURL, tlsConfig, true)
	if requestErr != nil {
		return requestErr
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("KubePrism upstream %q returned HTTP status %d from /readyz", address, statusCode)
	}

	return nil
}

func requestReadyz(ctx context.Context, checkURL string, tlsConfig *tls.Config, useClientCertificate bool) (int, error) {
	if !useClientCertificate {
		tlsConfig = tlsConfig.Clone()
		tlsConfig.Certificates = nil
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           nil,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return 0, err
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, fmt.Errorf("KubePrism readiness request failed: %w", err)
	}
	defer response.Body.Close()

	_, _ = io.Copy(io.Discard, response.Body)

	return response.StatusCode, nil
}

type connectionTracker struct {
	mu       sync.Mutex
	active   map[string]map[*trackedConnection]struct{}
	failures map[string]int
	closed   bool
}

type trackedConnection struct {
	once  sync.Once
	conns []net.Conn
}

func newConnectionTracker() *connectionTracker {
	return &connectionTracker{
		active:   make(map[string]map[*trackedConnection]struct{}),
		failures: make(map[string]int),
	}
}

func (tracker *connectionTracker) add(address string, conns ...net.Conn) *trackedConnection {
	tracked := &trackedConnection{conns: conns}

	tracker.mu.Lock()
	if tracker.closed {
		tracker.mu.Unlock()
		tracked.close()

		return nil
	}

	if tracker.active[address] == nil {
		tracker.active[address] = make(map[*trackedConnection]struct{})
	}

	tracker.active[address][tracked] = struct{}{}
	tracker.mu.Unlock()

	return tracked
}

func (tracker *connectionTracker) remove(address string, tracked *trackedConnection) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	delete(tracker.active[address], tracked)
	if len(tracker.active[address]) == 0 {
		delete(tracker.active, address)
	}
}

func (tracker *connectionTracker) recordFailure(address string) bool {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	tracker.failures[address]++

	return tracker.failures[address] == drainFailureThreshold
}

func (tracker *connectionTracker) recordSuccess(address string) {
	tracker.mu.Lock()
	tracker.failures[address] = 0
	tracker.mu.Unlock()
}

func (tracker *connectionTracker) drain(address string) {
	tracker.mu.Lock()
	connections := make([]*trackedConnection, 0, len(tracker.active[address]))
	for connection := range tracker.active[address] {
		connections = append(connections, connection)
	}
	tracker.mu.Unlock()

	for _, connection := range connections {
		connection.close()
	}
}

func (tracker *connectionTracker) closeAll() {
	tracker.mu.Lock()
	tracker.closed = true
	var connections []*trackedConnection
	for _, active := range tracker.active {
		for connection := range active {
			connections = append(connections, connection)
		}
	}
	tracker.mu.Unlock()

	for _, connection := range connections {
		connection.close()
	}
}

func (connection *trackedConnection) close() {
	connection.once.Do(func() {
		for _, conn := range connection.conns {
			_ = conn.Close()
		}
	})
}

func sliceSeq[T any](items []T) func(func(T) bool) {
	return func(yield func(T) bool) {
		for _, item := range items {
			if !yield(item) {
				return
			}
		}
	}
}
