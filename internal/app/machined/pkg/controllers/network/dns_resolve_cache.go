// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/ctxutil"
	"github.com/siderolabs/talos/internal/pkg/dns"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DNSResolveCacheController starts dns server on both udp and tcp ports based on finalized network configuration.
type DNSResolveCacheController struct {
	Addr   string
	Logger *zap.Logger
}

// Name implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Name() string {
	return "network.DNSResolveCacheController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DNSResolveCacheController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverStatusType,
			ID:        optional.Some(network.ResolverID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		mc, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !mc.Config().Machine().Features().LocalDNSEnabled() {
			continue
		}

		err = func() error {
			ctrl.Logger.Info("starting dns cache resolve")
			defer ctrl.Logger.Info("stopping dns cache resolve")

			return ctrl.runServer(ctx, r)
		}()
		if err != nil {
			return err
		}
	}
}

func (ctrl *DNSResolveCacheController) writeDNSStatus(ctx context.Context, r controller.Runtime, net resource.ID, handler *dns.Handler) error {
	return safe.WriterModify(ctx, r, network.NewDNSResolveCache(net), func(drc *network.DNSResolveCache) error {
		drc.TypedSpec().Status = "running"
		drc.TypedSpec().Servers = handler.ProxyList()

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
	addr := ctrl.Addr
	ctx := originCtx

	for _, opt := range []dns.ServerOptins{
		{
			Addr:    addr,
			Net:     "udp",
			Handler: cache,
		},
		{
			Addr:          addr,
			Net:           "tcp",
			Handler:       cache,
			ReadTimeout:   3 * time.Second,
			WriteTimeout:  5 * time.Second,
			IdleTimeout:   func() time.Duration { return 10 * time.Second },
			MaxTCPQueries: -1,
		},
	} {
		l := ctrl.Logger.With(zap.String("net", opt.Net))

		runner := dns.NewRunner(dns.NewServer(opt), l)

		err := ctrl.writeDNSStatus(ctx, r, opt.Net, handler)
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

		mc, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			return err
		}

		if !mc.Config().Machine().Features().LocalDNSEnabled() {
			return nil
		}

		resolverStatus, err := safe.ReaderGetByID[*network.ResolverStatus](ctx, r, network.ResolverID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting resolver status: %w", err)
		}

		ctrl.Logger.Info("updating dns server nameservers", zap.Stringers("data", resolverStatus.TypedSpec().DNSServers))

		err = handler.SetProxy(resolverStatus.TypedSpec().DNSServers)
		if err != nil {
			return fmt.Errorf("error setting dns server nameservers: %w", err)
		}

		for _, n := range []string{"udp", "tcp"} {
			err = ctrl.writeDNSStatus(ctx, r, n, handler)
			if err != nil {
				return err
			}
		}
	}
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
