// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// RouteStatusController manages secrets.Etcd based on configuration.
type RouteStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *RouteStatusController) Name() string {
	return "network.RouteStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RouteStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *RouteStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.RouteStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RouteStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	watcher, err := watch.NewRtNetlink(watch.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_IPV4_MROUTE|unix.RTMGRP_IPV4_ROUTE|unix.RTMGRP_IPV6_MROUTE|unix.RTMGRP_IPV6_ROUTE)
	if err != nil {
		return err
	}

	defer watcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// build links lookup table
		links, err := conn.Link.List()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		linkLookup := make(map[uint32]string, len(links))

		for _, link := range links {
			linkLookup[link.Index] = link.Attributes.Name
		}

		// list resources for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.RouteStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		itemsToDelete := map[resource.ID]struct{}{}

		for _, r := range list.Items {
			itemsToDelete[r.Metadata().ID()] = struct{}{}
		}

		routes, err := conn.Route.List()
		if err != nil {
			return fmt.Errorf("error listing routes: %w", err)
		}

		for _, route := range routes {
			dstAddr, _ := netip.AddrFromSlice(route.Attributes.Dst)
			dstPrefix := netip.PrefixFrom(dstAddr, int(route.DstLength))
			srcAddr, _ := netip.AddrFromSlice(route.Attributes.Src)
			gatewayAddr, _ := netip.AddrFromSlice(route.Attributes.Gateway)
			outLinkName := linkLookup[route.Attributes.OutIface]

			id := network.RouteID(nethelpers.RoutingTable(route.Table), nethelpers.Family(route.Family), dstPrefix, gatewayAddr, route.Attributes.Priority, outLinkName)

			if err = safe.WriterModify(ctx, r, network.NewRouteStatus(network.NamespaceName, id), func(r *network.RouteStatus) error {
				status := r.TypedSpec()

				status.Family = nethelpers.Family(route.Family)
				status.Destination = dstPrefix
				status.Source = srcAddr
				status.Gateway = gatewayAddr
				status.OutLinkIndex = route.Attributes.OutIface
				status.OutLinkName = outLinkName
				status.Priority = route.Attributes.Priority
				status.Table = nethelpers.RoutingTable(route.Table)
				status.Scope = nethelpers.Scope(route.Scope)
				status.Type = nethelpers.RouteType(route.Type)
				status.Protocol = nethelpers.RouteProtocol(route.Protocol)
				status.Flags = nethelpers.RouteFlags(route.Flags)

				if route.Attributes.Metrics != nil {
					status.MTU = route.Attributes.Metrics.MTU
				} else {
					status.MTU = 0
				}

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying resource: %w", err)
			}

			delete(itemsToDelete, id)
		}

		for id := range itemsToDelete {
			if err = r.Destroy(ctx, resource.NewMetadata(network.NamespaceName, network.RouteStatusType, id, resource.VersionUndefined)); err != nil {
				return fmt.Errorf("error deleting route status %q: %w", id, err)
			}
		}

		r.ResetRestartBackoff()
	}
}
