// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/pair"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/dns"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DNSResolveCacheController starts dns server on both udp and tcp ports based on finalized network configuration.
type DNSResolveCacheController struct {
	Logger *zap.Logger

	mx          sync.Mutex
	handler     *dns.Handler
	cache       *dns.Cache
	runners     map[runnerConfig]pair.Pair[func(), <-chan struct{}]
	reconcile   chan struct{}
	originalCtx context.Context //nolint:containedctx
}

// Name implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Name() string {
	return "network.DNSResolveCacheController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Inputs() []controller.Input {
	return []controller.Input{
		safe.Input[*network.DNSUpstream](controller.InputWeak),
		{
			Namespace: network.NamespaceName,
			Type:      network.HostDNSConfigType,
			ID:        optional.Some(network.HostDNSConfigID),
			Kind:      controller.InputWeak,
		},
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
//
//nolint:gocyclo,cyclop
func (ctrl *DNSResolveCacheController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.init(ctx)

	ctrl.mx.Lock()
	defer ctrl.mx.Unlock()

	defer ctrl.stopRunners(ctx, false)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ctrl.reconcile:
			for cfg, stop := range ctrl.runners {
				select {
				default:
					continue
				case <-stop.F2:
				}

				stop.F1()
				delete(ctrl.runners, cfg)
			}
		}

		cfg, err := safe.ReaderGetByID[*network.HostDNSConfig](ctx, r, network.HostDNSConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting host dns config: %w", err)
		}

		r.StartTrackingOutputs()

		if !cfg.TypedSpec().Enabled {
			ctrl.stopRunners(ctx, true)

			if err = safe.CleanupOutputs[*network.DNSResolveCache](ctx, r); err != nil {
				return fmt.Errorf("error cleaning up dns status on disable: %w", err)
			}

			continue
		}

		touchedRunners := make(map[runnerConfig]struct{}, len(ctrl.runners))

		for _, addr := range cfg.TypedSpec().ListenAddresses {
			for _, netwk := range []string{"udp", "tcp"} {
				runnerCfg := runnerConfig{net: netwk, addr: addr}

				if _, ok := ctrl.runners[runnerCfg]; !ok {
					runner, rErr := newDNSRunner(runnerCfg, ctrl.cache, ctrl.Logger)
					if rErr != nil {
						return fmt.Errorf("error creating dns runner: %w", rErr)
					}

					ctrl.runners[runnerCfg] = pair.MakePair(runner.Start(ctrl.handleDone(ctx, logger)))
				}

				if err = ctrl.writeDNSStatus(ctx, r, runnerCfg); err != nil {
					return fmt.Errorf("error writing dns status: %w", err)
				}

				touchedRunners[runnerCfg] = struct{}{}
			}
		}

		for runnerCfg, stop := range ctrl.runners {
			if _, ok := touchedRunners[runnerCfg]; !ok {
				stop.F1()
				delete(ctrl.runners, runnerCfg)

				continue
			}
		}

		upstreams, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
		if err != nil {
			return fmt.Errorf("error getting resolver status: %w", err)
		}

		addrs, prxs := make([]string, 0, upstreams.Len()), make([]*proxy.Proxy, 0, upstreams.Len())

		for it := upstreams.Iterator(); it.Next(); {
			prx := it.Value().TypedSpec().Value.Prx

			addrs = append(addrs, prx.Addr())
			prxs = append(prxs, prx.(*proxy.Proxy)) //nolint:forcetypeassert
		}

		if ctrl.handler.SetProxy(prxs) {
			ctrl.Logger.Info("updated dns server nameservers", zap.Strings("addrs", addrs))
		}

		if err = safe.CleanupOutputs[*network.DNSResolveCache](ctx, r); err != nil {
			return fmt.Errorf("error cleaning up dns status: %w", err)
		}
	}
}

func (ctrl *DNSResolveCacheController) writeDNSStatus(ctx context.Context, r controller.Runtime, config runnerConfig) error {
	return safe.WriterModify(ctx, r, network.NewDNSResolveCache(fmt.Sprintf("%s-%s", config.net, config.addr)), func(drc *network.DNSResolveCache) error {
		drc.TypedSpec().Status = "running"

		return nil
	})
}

func (ctrl *DNSResolveCacheController) init(ctx context.Context) {
	if ctrl.runners != nil {
		if ctrl.originalCtx != ctx {
			// This should not happen, but if it does, it's a bug.
			panic("DNSResolveCacheController is called with a different context")
		}

		return
	}

	ctrl.originalCtx = ctx
	ctrl.handler = dns.NewHandler(ctrl.Logger)
	ctrl.cache = dns.NewCache(ctrl.handler, ctrl.Logger)
	ctrl.runners = map[runnerConfig]pair.Pair[func(), <-chan struct{}]{}
	ctrl.reconcile = make(chan struct{}, 1)

	// Ensure we stop all runners when the context is canceled, no matter where we are currently.
	// For example if we are in Controller runtime sleeping after error and ctx is canceled, we should stop all runners
	// but, we will never call Run method again, so we need to ensure this happens regardless of the current state.
	context.AfterFunc(ctx, func() {
		ctrl.mx.Lock()
		defer ctrl.mx.Unlock()

		ctrl.stopRunners(ctx, true)
	})
}

func (ctrl *DNSResolveCacheController) stopRunners(ctx context.Context, ignoreCtx bool) {
	if !ignoreCtx && ctx.Err() == nil {
		// context not yet canceled, preserve runners, cache and handler
		return
	}

	for _, stop := range ctrl.runners {
		stop.F1()
	}

	clear(ctrl.runners)

	ctrl.handler.Stop()
}

func (ctrl *DNSResolveCacheController) handleDone(ctx context.Context, logger *zap.Logger) func(err error) {
	return func(err error) {
		if ctx.Err() != nil {
			if err != nil && !errors.Is(err, net.ErrClosed) {
				logger.Error("controller is closing, but error running dns server", zap.Error(err))
			}

			return
		}

		if err != nil {
			logger.Error("error running dns server", zap.Error(err))
		}

		select {
		case ctrl.reconcile <- struct{}{}:
		default:
		}
	}
}

type runnerConfig struct {
	net  string
	addr netip.AddrPort
}

func newDNSRunner(cfg runnerConfig, cache *dns.Cache, logger *zap.Logger) (*dns.Server, error) {
	if cfg.addr.Addr().Is6() {
		cfg.net += "6"
	}

	logger = logger.With(zap.String("net", cfg.net), zap.Stringer("addr", cfg.addr))

	var serverOpts dns.ServerOptions

	switch cfg.net {
	case "udp", "udp6":
		packetConn, err := dns.NewUDPPacketConn(cfg.net, cfg.addr.String())
		if err != nil {
			return nil, fmt.Errorf("error creating %q packet conn: %w", cfg.net, err)
		}

		serverOpts = dns.ServerOptions{
			PacketConn: packetConn,
			Handler:    cache,
			Logger:     logger,
		}

	case "tcp", "tcp6":
		listener, err := dns.NewTCPListener(cfg.net, cfg.addr.String())
		if err != nil {
			return nil, fmt.Errorf("error creating %q listener: %w", cfg.net, err)
		}

		serverOpts = dns.ServerOptions{
			Listener:      listener,
			Handler:       cache,
			ReadTimeout:   3 * time.Second,
			WriteTimeout:  5 * time.Second,
			IdleTimeout:   func() time.Duration { return 10 * time.Second },
			MaxTCPQueries: -1,
			Logger:        logger,
		}
	}

	return dns.NewServer(serverOpts), nil
}
