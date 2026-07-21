// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/mdlayher/ndp"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// raInterval is how often Router Advertisements are emitted on each unnumbered link.
const raInterval = 10 * time.Second

var allNodesMulticast = netip.MustParseAddr("ff02::1")

// RouterAdvertisementController sends IPv6 Router Advertisements on links used for unnumbered
// BGP peering, so the neighbor can discover this node's link-local address (gobgp's neighbor-interface
// relies on the kernel neighbor table, which the Linux kernel does not populate by sending RAs itself).
//
// It also enables net.ipv6.conf.<iface>.accept_ra=2 on those links (accept RAs while forwarding)
// so this node learns the neighbor's link-local in return.
type RouterAdvertisementController struct {
	senders map[string]context.CancelFunc
}

// Name implements controller.Controller interface.
func (ctrl *RouterAdvertisementController) Name() string {
	return "network.RouterAdvertisementController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RouterAdvertisementController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.BGPInstanceConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RouterAdvertisementController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.KernelParamDefaultSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *RouterAdvertisementController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.senders = map[string]context.CancelFunc{}

	defer ctrl.stopAll()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *RouterAdvertisementController) stopAll() {
	for _, cancel := range ctrl.senders {
		cancel()
	}

	ctrl.senders = map[string]context.CancelFunc{}
}

//nolint:gocyclo
func (ctrl *RouterAdvertisementController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	configResources, err := safe.ReaderListAll[*network.BGPInstanceConfig](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing BGP instance configs: %w", err)
	}

	linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing link statuses: %w", err)
	}

	linkResolver := network.NewLinkResolver(linkStatuses.All)
	readyLinks := map[string]struct{}{}

	for linkStatus := range linkStatuses.All() {
		readyLinks[linkStatus.Metadata().ID()] = struct{}{}
	}

	interfaces := map[string]struct{}{}

	for configResource := range configResources.All() {
		for _, neighbor := range configResource.TypedSpec().Neighbors {
			link := linkResolver.Resolve(neighbor.Link)
			if _, ready := readyLinks[link]; ready {
				interfaces[link] = struct{}{}
			}
		}
	}

	// reconcile RA-sender goroutines: start for new interfaces, stop for removed ones.
	for iface, cancel := range ctrl.senders {
		if _, ok := interfaces[iface]; !ok {
			cancel()
			delete(ctrl.senders, iface)
		}
	}

	for iface := range interfaces {
		if _, ok := ctrl.senders[iface]; ok {
			continue
		}

		senderCtx, cancel := context.WithCancel(ctx)
		ctrl.senders[iface] = cancel

		go ctrl.runSender(senderCtx, iface, logger)
	}

	// enable accept_ra=2 on each unnumbered interface so we learn the neighbor's link-local.
	r.StartTrackingOutputs()

	for _, iface := range sortedKeys(interfaces) {
		id := kernel.Sysctl + "." + fmt.Sprintf("net/ipv6/conf/%s/accept_ra", iface)

		if err = safe.WriterModify(ctx, r, runtimeres.NewKernelParamDefaultSpec(runtimeres.NamespaceName, id), func(spec *runtimeres.KernelParamDefaultSpec) error {
			spec.TypedSpec().Value = "2"
			spec.TypedSpec().IgnoreErrors = true

			return nil
		}); err != nil {
			return fmt.Errorf("error setting accept_ra: %w", err)
		}
	}

	return safe.CleanupOutputs[*runtimeres.KernelParamDefaultSpec](ctx, r)
}

// runSender periodically emits Router Advertisements on the given interface until the context is done.
func (ctrl *RouterAdvertisementController) runSender(ctx context.Context, iface string, logger *zap.Logger) {
	ticker := time.NewTicker(raInterval)
	defer ticker.Stop()

	for {
		if err := ctrl.sendOnce(iface); err != nil {
			logger.Debug("failed to send router advertisement", zap.String("interface", iface), zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (ctrl *RouterAdvertisementController) sendOnce(iface string) error {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return err
	}

	conn, _, err := ndp.Listen(ifi, ndp.LinkLocal)
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	// RouterLifetime 0: advertise presence for link-local discovery only, not as a default router.
	// The Source Link-Layer Address option lets the peer populate a complete (resolvable) neighbor
	// cache entry for our link-local, which gobgp's unnumbered peering relies on.
	ra := &ndp.RouterAdvertisement{
		CurrentHopLimit: 64,
		RouterLifetime:  0,
		Options: []ndp.Option{
			&ndp.LinkLayerAddress{
				Direction: ndp.Source,
				Addr:      ifi.HardwareAddr,
			},
		},
	}

	return conn.WriteTo(ra, nil, allNodesMulticast)
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
