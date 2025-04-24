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
func NewRunner(opts RunnerOptions, l *zap.Logger) *Runner {
	return &Runner{
		srv: &dns.Server{
			Listener:      opts.Listener,
			PacketConn:    opts.PacketConn,
			Handler:       opts.Handler,
			UDPSize:       dns.DefaultMsgSize, // 4096 since default is [dns.MinMsgSize] = 512 bytes, which is too small.
			ReadTimeout:   opts.ReadTimeout,
			WriteTimeout:  opts.WriteTimeout,
			IdleTimeout:   opts.IdleTimeout,
			MaxTCPQueries: opts.MaxTCPQueries,
		},
		logger: l,
	}
}

// Runner is a DNS server runner.
type Runner struct {
	srv    *dns.Server
	logger *zap.Logger
}

// Serve starts the DNS server. Implements [suture.Service] interface.
func (r *Runner) Serve(ctx context.Context) error {
	errCh := make(chan error)

	go func() {
		errCh <- r.srv.ActivateAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	r.close()

	select {
	case err := <-errCh:
		return err
	case <-time.After(time.Second):
		return errors.New("timeout waiting for server to close")
	}
}

func (r *Runner) close() {
	l := r.logger

	if r.srv.Listener != nil {
		l = l.With(zap.String("net", "tcp"), zap.String("local_addr", r.srv.Listener.Addr().String()))
	} else if r.srv.PacketConn != nil {
		l = l.With(zap.String("net", "udp"), zap.String("local_addr", r.srv.PacketConn.LocalAddr().String()))
	}

	closer := io.Closer(r.srv.Listener)
	if closer == nil {
		closer = r.srv.PacketConn
	}

	if closer != nil {
		if err := closer.Close(); err != nil {
			l.Error("error closing dns server listener", zap.Error(err))
		} else {
			l.Debug("dns server listener closed")
		}
	}

	sCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sCancel()

	err := r.srv.ShutdownContext(sCtx)
	if err != nil && !errors.Is(err, net.ErrClosed) {
		l.Error("error shutting down dns server", zap.Error(err))
	}
}
