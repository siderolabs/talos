// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/jsimonetti/rtnetlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/resources/network"
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
func (ctrl *RouteStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	watcher, err := watch.NewRtNetlink(r, unix.RTMGRP_IPV4_MROUTE|unix.RTMGRP_IPV4_ROUTE|unix.RTMGRP_IPV6_MROUTE|unix.RTMGRP_IPV6_ROUTE)
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
			route := route

			dstAddr, _ := netaddr.FromStdIPRaw(route.Attributes.Dst)
			dstPrefix := netaddr.IPPrefixFrom(dstAddr, route.DstLength)
			srcAddr, _ := netaddr.FromStdIPRaw(route.Attributes.Src)
			srcPrefix := netaddr.IPPrefixFrom(srcAddr, route.SrcLength)
			gatewayAddr, _ := netaddr.FromStdIPRaw(route.Attributes.Gateway)
			id := network.RouteID(nethelpers.RoutingTable(route.Table), nethelpers.Family(route.Family), dstPrefix, gatewayAddr, route.Attributes.Priority)

			if err = r.Modify(ctx, network.NewRouteStatus(network.NamespaceName, id), func(r resource.Resource) error {
				status := r.(*network.RouteStatus).TypedSpec()

				status.Family = nethelpers.Family(route.Family)
				status.Destination = dstPrefix
				status.Source = srcPrefix
				status.Gateway = gatewayAddr
				status.OutLinkIndex = route.Attributes.OutIface
				status.OutLinkName = linkLookup[route.Attributes.OutIface]
				status.Priority = route.Attributes.Priority
				status.Table = nethelpers.RoutingTable(route.Table)
				status.Scope = nethelpers.Scope(route.Scope)
				status.Type = nethelpers.RouteType(route.Type)
				status.Protocol = nethelpers.RouteProtocol(route.Protocol)
				status.Flags = nethelpers.RouteFlags(route.Flags)

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
	}
}
