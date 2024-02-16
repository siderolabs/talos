// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"net"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DNSUpstreamController is a controller that manages DNS upstreams.
type DNSUpstreamController struct{}

// Name implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Name() string {
	return "network.DNSUpstreamController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.ResolverStatusType,
			ID:        optional.Some(network.ResolverID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.DNSUpstreamType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *DNSUpstreamController) Run(ctx context.Context, r controller.Runtime, l *zap.Logger) error {
	defer ctrl.cleanupUpstream(context.Background(), r, nil, l)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.run(ctx, r, l); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *DNSUpstreamController) run(ctx context.Context, r controller.Runtime, l *zap.Logger) error {
	touchedIDs := map[resource.ID]struct{}{}

	defer ctrl.cleanupUpstream(ctx, r, touchedIDs, l)

	mc, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	machineConfig := mc.Config().Machine()
	if machineConfig == nil || !machineConfig.Features().LocalDNSEnabled() {
		return nil
	}

	rs, err := safe.ReaderGetByID[*network.ResolverStatus](ctx, r, network.ResolverID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	for _, s := range rs.TypedSpec().DNSServers {
		remoteAddr := s.String()

		if err = safe.WriterModify[*network.DNSUpstream](
			ctx,
			r,
			network.NewDNSUpstream(remoteAddr),
			func(u *network.DNSUpstream) error {
				touchedIDs[u.Metadata().ID()] = struct{}{}

				if u.TypedSpec().Value.Prx != nil {
					return nil
				}

				prx := proxy.NewProxy(remoteAddr, net.JoinHostPort(remoteAddr, "53"), "dns")

				prx.Start(500 * time.Millisecond)

				u.TypedSpec().Value.Prx = prx

				l.Info("created dns upstream", zap.String("addr", remoteAddr))

				return nil
			},
		); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *DNSUpstreamController) cleanupUpstream(ctx context.Context, r controller.Runtime, touchedIDs map[resource.ID]struct{}, l *zap.Logger) {
	list, err := safe.ReaderListAll[*network.DNSUpstream](ctx, r)
	if err != nil {
		l.Error("error listing upstreams", zap.Error(err))

		return
	}

	for it := list.Iterator(); it.Next(); {
		val := it.Value()
		md := val.Metadata()

		if _, ok := touchedIDs[md.ID()]; !ok {
			val.TypedSpec().Value.Prx.Stop()

			if err = r.Destroy(ctx, md); err != nil {
				l.Error("error destroying upstream", zap.Error(err), zap.String("id", md.ID()))

				return
			}

			l.Info("destroyed dns upstream", zap.String("addr", md.ID()))
		}
	}
}
