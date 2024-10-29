// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/xcontext"
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

// Serve starts the DNS server.
func (r *Runner) Serve(ctx context.Context) error {
	detach := xcontext.AfterFuncSync(ctx, r.close)
	defer func() {
		if !detach() {
			return
		}

		r.close()
	}()

	return r.srv.ActivateAndServe()
}

func (r *Runner) close() {
	l := r.logger

	if r.srv.Listener != nil {
		l = l.With(zap.String("net", "tcp"), zap.String("local_addr", r.srv.Listener.Addr().String()))
	} else if r.srv.PacketConn != nil {
		l = l.With(zap.String("net", "udp"), zap.String("local_addr", r.srv.PacketConn.LocalAddr().String()))
	}

	for {
		err := r.srv.Shutdown()
		if err != nil {
			if strings.Contains(err.Error(), "server not started") {
				// There a possible scenario where `go func()` not yet reached `ActivateAndServe` and yielded CPU
				// time to another goroutine and then this closure reached `Shutdown`. In that case
				// `dns.Server.ActivateAndServe` will actually start after `Shutdown` and this closure will block forever
				// because `go func()` will never exit and close `done` channel.
				continue
			}

			l.Error("error shutting down dns server", zap.Error(err))
		}

		closer := io.Closer(r.srv.Listener)
		if closer == nil {
			closer = r.srv.PacketConn
		}

		if closer != nil {
			err = closer.Close()
			if err != nil && !errors.Is(err, net.ErrClosed) {
				l.Error("error closing dns server listener", zap.Error(err))
			} else {
				l.Debug("dns server listener closed")
			}
		}

		break
	}
}
