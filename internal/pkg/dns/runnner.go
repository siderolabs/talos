// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

// RunnerOptions is a [Runner] options.
type RunnerOptions struct {
	Listener      net.Listener
	PacketConn    net.PacketConn
	Handler       dns.Handler
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   func() time.Duration
	MaxTCPQueries int
}

// NewRunner creates a new [Runner].
//
// newOpts is invoked on every (re)start to build a fresh [dns.Server] with
// fresh listeners. Both are single-use: once [dns.Server.ActivateAndServe]
// returns it leaves the server marked as started and miekg/dns closes the
// underlying listener/packet conn. Reusing the same instances across a
// supervisor restart therefore fails permanently with
// "dns: server already started", so the runner must reconstruct them.
func NewRunner(newOpts func() (RunnerOptions, error), l *zap.Logger) *Runner {
	return &Runner{
		newOpts: newOpts,
		logger:  l,
	}
}

// Runner is a DNS server runner.
type Runner struct {
	newOpts func() (RunnerOptions, error)
	logger  *zap.Logger
}

// Serve starts the DNS server. Implements [suture.Service] interface.
func (r *Runner) Serve(ctx context.Context) error {
	opts, err := r.newOpts()
	if err != nil {
		return err
	}

	srv := &dns.Server{
		Listener:      opts.Listener,
		PacketConn:    opts.PacketConn,
		Handler:       opts.Handler,
		UDPSize:       dns.DefaultMsgSize, // 4096 since default is [dns.MinMsgSize] = 512 bytes, which is too small.
		ReadTimeout:   opts.ReadTimeout,
		WriteTimeout:  opts.WriteTimeout,
		IdleTimeout:   opts.IdleTimeout,
		MaxTCPQueries: opts.MaxTCPQueries,
	}

	// Buffered so the goroutine never blocks on send if Serve returns early
	// (e.g. via the close timeout below), avoiding a leaked goroutine.
	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.ActivateAndServe()
	}()

	select {
	case err := <-errCh:
		// ActivateAndServe returned on its own (a serve error, or an early
		// return such as a setUDPSocketOptions failure). The server has already
		// stopped, but on some early-return paths miekg/dns does not close the
		// freshly-bound listener/conn, so close it here to avoid leaking the
		// socket across restarts. ShutdownContext is not needed: the server is
		// already stopped, and calling it would only report "server not
		// started" on the paths that never marked it started.
		r.closeListener(srv)

		return err
	case <-ctx.Done():
	}

	// Graceful stop requested: shut the server down and wait for the serve
	// goroutine to return.
	r.shutdown(srv)

	select {
	case err := <-errCh:
		return err
	case <-time.After(time.Second):
		return errors.New("timeout waiting for server to close")
	}
}

// loggerFor returns the runner logger annotated with the server's network and
// local address.
func (r *Runner) loggerFor(srv *dns.Server) *zap.Logger {
	switch {
	case srv.Listener != nil:
		return r.logger.With(zap.String("net", "tcp"), zap.String("local_addr", srv.Listener.Addr().String()))
	case srv.PacketConn != nil:
		return r.logger.With(zap.String("net", "udp"), zap.String("local_addr", srv.PacketConn.LocalAddr().String()))
	default:
		return r.logger
	}
}

// closeListener closes the server's listener/packet conn, treating an
// already-closed socket (e.g. closed by the serve loop on its own return) as a
// no-op.
func (r *Runner) closeListener(srv *dns.Server) {
	closer := io.Closer(srv.Listener)
	if closer == nil {
		closer = srv.PacketConn
	}

	if closer == nil {
		return
	}

	switch err := closer.Close(); {
	case err == nil:
		r.loggerFor(srv).Debug("dns server listener closed")
	case errors.Is(err, net.ErrClosed):
		// Already closed; nothing to do.
	default:
		r.loggerFor(srv).Error("error closing dns server listener", zap.Error(err))
	}
}

// shutdown gracefully stops a running server: it closes the listener and waits
// for in-flight queries to drain.
func (r *Runner) shutdown(srv *dns.Server) {
	r.closeListener(srv)

	sCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sCancel()

	if err := srv.ShutdownContext(sCtx); err != nil && !errors.Is(err, net.ErrClosed) {
		r.loggerFor(srv).Error("error shutting down dns server", zap.Error(err))
	}
}
