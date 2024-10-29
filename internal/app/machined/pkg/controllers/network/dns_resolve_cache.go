// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"net/netip"
	"sync"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xiter"
	"github.com/thejerf/suture/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/pkg/dns"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DNSResolveCacheController starts dns server on both udp and tcp ports based on finalized network configuration.
type DNSResolveCacheController struct {
	State  state.State
	Logger *zap.Logger

	mx        sync.Mutex
	manager   *dns.Manager
	reconcile chan struct{}
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
func (ctrl *DNSResolveCacheController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.init(ctx)

	ctrl.mx.Lock()
	defer ctrl.mx.Unlock()

	defer func() {
		if err := ctrl.manager.ClearAll(ctx.Err() == nil); err != nil {
			ctrl.Logger.Error("error stopping dns runners", zap.Error(err))
		}

		if ctx.Err() != nil {
			ctrl.Logger.Info("manager finished", zap.Error(<-ctrl.manager.Done()))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ctrl.reconcile:
		}

		if err := ctrl.run(ctx, r); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *DNSResolveCacheController) run(ctx context.Context, r controller.Runtime) (resErr error) {
	r.StartTrackingOutputs()
	defer cleanupOutputs(ctx, r, &resErr)

	cfg, err := safe.ReaderGetByID[*network.HostDNSConfig](ctx, r, network.HostDNSConfigID)

	switch {
	case state.IsNotFoundError(err):
		return nil
	case err != nil:
		return fmt.Errorf("error getting host dns config: %w", err)
	}

	ctrl.manager.AllowNodeResolving(cfg.TypedSpec().ResolveMemberNames)

	if !cfg.TypedSpec().Enabled {
		return ctrl.manager.ClearAll(false)
	}

	pairs := allAddressPairs(cfg.TypedSpec().ListenAddresses)
	forwardKubeDNSToHost := cfg.TypedSpec().ServiceHostDNSAddress.IsValid()

	for runCfg, runErr := range ctrl.manager.RunAll(pairs, forwardKubeDNSToHost) {
		switch {
		case runErr != nil && (runCfg.Network == "tcp6" || runCfg.Network == "udp6"):
			// Ignore ipv6 errors
			ctrl.Logger.Warn("ignoring ipv6 dns runner error", zap.Error(runErr))
		case runErr != nil:
			return fmt.Errorf("error updating dns runner '%v': %w", runCfg, runErr)
		case runCfg.Status == dns.StatusRemoved:
			// Removed runned, no reason to update status
			continue
		}

		if err = ctrl.writeDNSStatus(ctx, r, runCfg.AddressPair); err != nil {
			return fmt.Errorf("error writing dns status: %w", err)
		}
	}

	upstreams, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
	if err != nil {
		return fmt.Errorf("error getting resolver status: %w", err)
	}

	prxs := xiter.Map(
		// We are using iterator here to preserve finalizer on
		func(upstream *network.DNSUpstream) *proxy.Proxy {
			return upstream.TypedSpec().Value.Conn.Proxy().(*proxy.Proxy)
		},
		upstreams.All(),
	)

	if ctrl.manager.SetUpstreams(prxs) {
		ctrl.Logger.Info("updated dns server nameservers", zap.Array("addrs", addrsArr(upstreams)))
	}

	return nil
}

func cleanupOutputs(ctx context.Context, r controller.Runtime, resErr *error) {
	if err := safe.CleanupOutputs[*network.DNSResolveCache](ctx, r); err != nil {
		*resErr = cmp.Or(*resErr, fmt.Errorf("error cleaning up dns resolve cache: %w", err))
	}
}

func (ctrl *DNSResolveCacheController) writeDNSStatus(ctx context.Context, r controller.Runtime, config dns.AddressPair) error {
	res := network.NewDNSResolveCache(fmt.Sprintf("%s-%s", config.Network, config.Addr))

	return safe.WriterModify(ctx, r, res, func(drc *network.DNSResolveCache) error {
		drc.TypedSpec().Status = "running"

		return nil
	})
}

func (ctrl *DNSResolveCacheController) init(ctx context.Context) {
	if ctrl.manager == nil {
		ctrl.manager = dns.NewManager(&memberReader{st: ctrl.State}, ctrl.eventHook, ctrl.Logger)

		// Ensure we stop all runners when the context is canceled, no matter where we are currently.
		// For example if we are in Controller runtime sleeping after error and ctx is canceled, we should stop all runners
		// but, we will never call Run method again, so we need to ensure this happens regardless of the current state.
		context.AfterFunc(ctx, func() {
			ctrl.mx.Lock()
			defer ctrl.mx.Unlock()

			if err := ctrl.manager.ClearAll(false); err != nil {
				ctrl.Logger.Error("error ctx stopping dns runners", zap.Error(err))
			}
		})
	}

	ctrl.manager.ServeBackground(ctx)
}

func (ctrl *DNSResolveCacheController) eventHook(event suture.Event) {
	ctrl.Logger.Info("dns-resolve-cache-runners event", zap.String("event", event.String()))

	select {
	case ctrl.reconcile <- struct{}{}:
	default:
	}
}

type memberReader struct{ st state.State }

func (m *memberReader) ReadMembers(ctx context.Context) (iter.Seq[*cluster.Member], error) {
	list, err := safe.ReaderListAll[*cluster.Member](ctx, m.st)
	if err != nil {
		return nil, err
	}

	return list.All(), nil
}

type addrsArr safe.List[*network.DNSUpstream]

func (a addrsArr) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	list := safe.List[*network.DNSUpstream](a)

	for u := range list.All() {
		encoder.AppendString(u.TypedSpec().Value.Conn.Addr())
	}

	return nil
}

func allAddressPairs(addresses []netip.AddrPort) iter.Seq[dns.AddressPair] {
	return func(yield func(dns.AddressPair) bool) {
		for _, addr := range addresses {
			networks := [...]string{"udp", "tcp"}
			if addr.Addr().Is6() {
				networks = [...]string{"udp6", "tcp6"}
			}

			for _, netwk := range networks {
				if !yield(dns.AddressPair{
					Network: netwk,
					Addr:    addr,
				}) {
					return
				}
			}
		}
	}
}
