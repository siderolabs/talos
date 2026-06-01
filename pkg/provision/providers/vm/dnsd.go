// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision"
)

func dnsServe(ctx context.Context, server *dns.Server, log *slog.Logger) error {
	// IPv6 address takes longer to become ready, and therefore might need a retry
	err := retry.Constant(
		time.Minute,
		retry.WithUnits(time.Second),
	).RetryWithContext(ctx, func(_ context.Context) error {
		log.Info("starting DNS forwarder server")

		err := server.ListenAndServe()
		if err != nil {
			log.Warn("Failed to listen", slog.String("err", err.Error()))
		}

		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			return retry.ExpectedErrorf("address %s unavailable", server.Addr)
		}

		return err
	})

	if ctx.Err() != nil {
		return nil //nolint:nilerr
	}

	return err
}

// DNSd entrypoint.
func DNSd(ctx context.Context, ips []net.IP, resolvConfPath string) error {
	eg, egCtx := errgroup.WithContext(ctx)

	config, err := dns.ClientConfigFromFile(resolvConfPath)
	if err != nil {
		return fmt.Errorf("failed to read %q: %v", resolvConfPath, err)
	}

	initLog := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	defer initLog.Info("bye")

	for _, ip := range ips {
		for _, mode := range []string{"udp", "tcp"} {
			addr := net.JoinHostPort(ip.String(), "53")
			log := initLog.With("mode", mode, "addr", addr)

			server := &dns.Server{
				Addr:    addr,
				Net:     mode,
				Handler: dns.HandlerFunc(forwardHandler(mode, log, config)),
			}

			eg.Go(func() error {
				<-egCtx.Done()

				sCtx, sCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer sCancel()

				if err := server.ShutdownContext(sCtx); err != nil && !errors.Is(err, net.ErrClosed) {
					return err
				}

				return nil
			})

			eg.Go(func() error {
				return dnsServe(egCtx, server, log)
			})
		}
	}

	return eg.Wait()
}

const (
	dnsPid = "dnsd.pid"
	dnsLog = "dnsd.log"
)

// CreateDNSd creates the DNSd server.
func (p *Provisioner) CreateDNSd(state *State, clusterReq provision.ClusterRequest) error {
	return p.startDNSd(state, clusterReq)
}

// DestroyDNSd destoys DNSd server.
func (p *Provisioner) DestroyDNSd(state *State) error {
	return p.stopDNSd(state)
}

func forwardHandler(mode string, log *slog.Logger, config *dns.ClientConfig) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		slog.Debug("handling DNS request", "request", r.String())

		c := new(dns.Client)
		c.Net = mode

		var (
			resp *dns.Msg
			err  error
		)

		for _, serverAddr := range config.Servers {
			log.Debug("making DNS request", "target", serverAddr+":"+config.Port)

			resp, _, err = c.Exchange(r, net.JoinHostPort(serverAddr, config.Port))
			if err != nil {
				log.Debug("DNS request failed", "error", err)

				continue
			}

			if resp != nil && (resp.Rcode == dns.RcodeServerFailure || resp.Rcode == dns.RcodeRefused) {
				log.Debug("DNS request succeeded, but RCODE reports failure", "response", resp.String())

				continue
			}

			break
		}

		if resp == nil {
			log.Error("DNS exchange failed", "error", err)
			dns.HandleFailed(w, r)

			return
		}

		log.Debug("writing DNS response", "response", resp.String())

		if err := w.WriteMsg(resp); err != nil {
			log.Error("failed to write response", "error", err)
		}
	}
}
