// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubeprism implements KubePrism upstream health checks.
package kubeprism

import (
	"bufio"
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/siderolabs/crypto/x509"
)

// maxErrorBodySize limits the part of the response body included into the error message.
const maxErrorBodySize = 512

// ReadinessChecker checks the readiness of the Kubernetes API server via the `/readyz` endpoint.
//
// Each check opens a new connection to the upstream and closes it before returning,
// so no connections are pooled between the checks.
//
// ReadinessChecker is safe for concurrent use, and the TLS configuration can be updated
// while the checks are running.
type ReadinessChecker struct {
	tlsConfig atomic.Pointer[tls.Config]
}

// Update the CAs and the server name used to verify the API server certificate.
func (checker *ReadinessChecker) Update(acceptedCAs []*x509.PEMEncodedCertificate, serverName string) error {
	pool := stdx509.NewCertPool()

	for _, ca := range acceptedCAs {
		if !pool.AppendCertsFromPEM(ca.Crt) {
			return errors.New("failed to parse Kubernetes CA certificate")
		}
	}

	if len(acceptedCAs) == 0 {
		return errors.New("no Kubernetes CA certificates")
	}

	checker.tlsConfig.Store(&tls.Config{
		RootCAs:    pool,
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"http/1.1"},
	})

	return nil
}

// Check the readiness of the API server at the address (host:port).
//
// Check implements loadbalancer.HealthCheckFunc.
func (checker *ReadinessChecker) Check(ctx context.Context, address string) error {
	tlsConfig := checker.tlsConfig.Load()
	if tlsConfig == nil {
		return errors.New("readiness check is not configured")
	}

	conn, err := (&tls.Dialer{Config: tlsConfig}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	// abort any blocked read or write as soon as the context is done
	stop := context.AfterFunc(ctx, func() {
		conn.SetDeadline(time.Now()) //nolint:errcheck
	})
	defer stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, (&url.URL{Scheme: "https", Host: address, Path: "/readyz"}).String(), nil)
	if err != nil {
		return err
	}

	req.Close = true

	if err = req.Write(conn); err != nil {
		return fmt.Errorf("error sending readiness request: %w", checkCtxErr(ctx, err))
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return fmt.Errorf("error reading readiness response: %w", checkCtxErr(ctx, err))
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize)) //nolint:errcheck

		return fmt.Errorf("API server is not ready: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return nil
}

// checkCtxErr replaces the deadline error caused by the context cancellation with the context error.
func checkCtxErr(ctx context.Context, err error) error {
	var netErr net.Error

	if ctx.Err() != nil && errors.As(err, &netErr) && netErr.Timeout() {
		return context.Cause(ctx)
	}

	return err
}
