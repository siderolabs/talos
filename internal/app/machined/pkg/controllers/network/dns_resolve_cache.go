// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/ctxutil"
	"github.com/siderolabs/talos/internal/pkg/dns"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DNSResolveCacheController starts dns server on both udp and tcp ports based on finalized network configuration.
type DNSResolveCacheController struct {
	Addr   string
	AddrV6 string
	Logger *zap.Logger
}

// Name implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Name() string {
	return "network.DNSResolveCacheController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Inputs() []controller.Input {
	return []controller.Input{
		safe.Input[*network.DNSUpstream](controller.InputWeak),
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.DNSResolveCacheType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		upstreams, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
		if err != nil {
			return fmt.Errorf("error getting resolver status: %w", err)
		}

		if upstreams.Len() == 0 {
			continue
		}

		err = func() error {
			ctrl.Logger.Info("starting dns caching resolver")
			defer ctrl.Logger.Info("stopping dns caching resolver")

			return ctrl.runServer(ctx, r)
		}()
		if err != nil {
			return err
		}
	}
}

func (ctrl *DNSResolveCacheController) writeDNSStatus(ctx context.Context, r controller.Runtime, net resource.ID) error {
	return safe.WriterModify(ctx, r, network.NewDNSResolveCache(net), func(drc *network.DNSResolveCache) error {
		drc.TypedSpec().Status = "running"

		return nil
	})
}

//nolint:gocyclo
func (ctrl *DNSResolveCacheController) runServer(originCtx context.Context, r controller.Runtime) error {
	defer func() {
		err := dropResolveResources(context.Background(), r, "tcp", "udp")
		if err != nil {
			ctrl.Logger.Error("error setting back the initial status", zap.Error(err))
		}
	}()

	handler := dns.NewHandler(ctrl.Logger)
	defer handler.Stop()

	cache := dns.NewCache(handler, ctrl.Logger)
	ctx := originCtx

	serverOpts := map[string]dns.ServerOptins{}

	for _, opt := range []struct {
		net  string
		addr string
	}{
		{net: "udp", addr: ctrl.Addr},
		{net: "udp6", addr: ctrl.AddrV6},
		{net: "tcp", addr: ctrl.Addr},
		{net: "tcp6", addr: ctrl.AddrV6},
	} {
		l := ctrl.Logger.With(zap.String("net", opt.net), zap.String("addr", opt.addr))

		switch opt.net {
		case "udp", "udp6":
			packetConn, err := dns.NewUDPPacketConn(opt.net, opt.addr)
			if err != nil {
				if opt.net == "udp6" {
					// If we can't bind to ipv6, we can continue with ipv4
					continue
				}

				return fmt.Errorf("error creating udp packet conn: %w", err)
			}

			defer closeListener(packetConn, l)

			serverOpts[opt.net] = dns.ServerOptins{
				PacketConn: packetConn,
				Handler:    cache,
			}

		case "tcp", "tcp6":
			listener, err := dns.NewTCPListener(opt.net, opt.addr)
			if err != nil {
				if opt.net == "tcp6" {
					// If we can't bind to ipv6, we can continue with ipv4
					continue
				}

				return fmt.Errorf("error creating tcp listener: %w", err)
			}

			defer closeListener(listener, l)

			serverOpts[opt.net] = dns.ServerOptins{
				Listener:      listener,
				Handler:       cache,
				ReadTimeout:   3 * time.Second,
				WriteTimeout:  5 * time.Second,
				IdleTimeout:   func() time.Duration { return 10 * time.Second },
				MaxTCPQueries: -1,
			}
		}

		l.Info("dns listener created")
	}

	for netwk, opt := range serverOpts {
		l := ctrl.Logger.With(zap.String("net", netwk))

		runner := dns.NewRunner(dns.NewServer(opt), l)

		err := ctrl.writeDNSStatus(ctx, r, netwk)
		if err != nil {
			return err
		}

		// We attach here our goroutine to the context, so if goroutine exits for some reason,
		// context will be canceled too.
		ctx = ctxutil.MonitorFn(ctx, runner.Run)

		defer runner.Stop()
	}

	// Skip first iteration
	eventCh := closedCh

	for {
		select {
		case <-ctx.Done():
			return ctxutil.Cause(ctx)
		case <-eventCh:
		}

		eventCh = r.EventCh()

		upstreams, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
		if err != nil {
			return fmt.Errorf("error getting resolver status: %w", err)
		}

		if upstreams.Len() == 0 {
			return nil
		}

		addrs := make([]string, 0, upstreams.Len())
		prxs := make([]*proxy.Proxy, 0, len(addrs))

		for it := upstreams.Iterator(); it.Next(); {
			upstream := it.Value()

			addrs = append(addrs, upstream.TypedSpec().Value.Prx.Addr())
			prxs = append(prxs, upstream.TypedSpec().Value.Prx.(*proxy.Proxy)) //nolint:forcetypeassert
		}

		if handler.SetProxy(prxs) {
			ctrl.Logger.Info("updated dns server nameservers", zap.Strings("addrs", addrs))
		}

		for _, n := range []string{"udp", "tcp"} {
			err = ctrl.writeDNSStatus(ctx, r, n)
			if err != nil {
				return err
			}
		}
	}
}

func closeListener(lis io.Closer, l *zap.Logger) {
	if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		l.Error("error closing listener", zap.Error(err))
	}

	l.Info("dns listener closed")
}

func dropResolveResources(ctx context.Context, r controller.Runtime, nets ...resource.ID) error {
	for _, net := range nets {
		if err := r.Destroy(ctx, network.NewDNSResolveCache(net).Metadata()); err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error destroying dns resolve cache resource: %w", err)
		}
	}

	return nil
}

var closedCh = func() <-chan controller.ReconcileEvent {
	res := make(chan controller.ReconcileEvent)
	close(res)

	return res
}()
