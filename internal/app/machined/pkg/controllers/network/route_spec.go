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
	"github.com/talos-systems/talos/pkg/resources/network"
)

// RouteSpecController applies network.RouteSpec to the actual interfaces.
type RouteSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *RouteSpecController) Name() string {
	return "network.RouteSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RouteSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.RouteSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RouteSpecController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,dupl
func (ctrl *RouteSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// watch link changes as some routes might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(r, unix.RTMGRP_LINK)
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

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.RouteSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// list rtnetlink links (interfaces)
		links, err := conn.Link.List()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		// list rtnetlink routes
		routes, err := conn.Route.List()
		if err != nil {
			return fmt.Errorf("error listing addresses: %w", err)
		}

		// loop over route and make reconcile decision
		for _, res := range list.Items {
			route := res.(*network.RouteSpec) //nolint:forcetypeassert,errcheck

			if err = ctrl.syncRoute(ctx, r, logger, conn, links, routes, route); err != nil {
				return err
			}
		}
	}
}

func findRoutes(routes []rtnetlink.RouteMessage, destination netaddr.IPPrefix, gateway netaddr.IP) []*rtnetlink.RouteMessage {
	var result []*rtnetlink.RouteMessage //nolint:prealloc

	for i, route := range routes {
		if route.DstLength != destination.Bits {
			continue
		}

		if !route.Attributes.Dst.Equal(destination.IP.IPAddr().IP) {
			continue
		}

		if !route.Attributes.Gateway.Equal(gateway.IPAddr().IP) {
			continue
		}

		result = append(result, &routes[i])
	}

	return result
}

//nolint:gocyclo
func (ctrl *RouteSpecController) syncRoute(ctx context.Context, r controller.Runtime, logger *zap.Logger, conn *rtnetlink.Conn,
	links []rtnetlink.LinkMessage, routes []rtnetlink.RouteMessage, route *network.RouteSpec) error {
	linkIndex := resolveLinkName(links, route.Status().OutLinkName)

	destinationStr := route.Status().Destination.String()

	if route.Status().Destination.IsZero() {
		destinationStr = "default"
	}

	switch route.Metadata().Phase() {
	case resource.PhaseTearingDown:
		for _, existing := range findRoutes(routes, route.Status().Destination, route.Status().Gateway) {
			// delete route
			if err := conn.Route.Delete(existing); err != nil {
				return fmt.Errorf("error removing route: %w", err)
			}

			logger.Sugar().Infof("removed route to %s via %s (link %q)", destinationStr, route.Status().Gateway, route.Status().OutLinkName)
		}

		// now remove finalizer as address was deleted
		if err := r.RemoveFinalizer(ctx, route.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer: %w", err)
		}
	case resource.PhaseRunning:
		if linkIndex == 0 && route.Status().OutLinkName != "" {
			// route can't be created as link doesn't exist (yet), skip it
			return nil
		}

		matchFound := false

		for _, existing := range findRoutes(routes, route.Status().Destination, route.Status().Gateway) {
			// check if existing matches the spec: if it does, skip update
			if existing.Scope == uint8(route.Status().Scope) && existing.Flags == uint32(route.Status().Flags) &&
				existing.Protocol == uint8(route.Status().Protocol) && existing.Flags == uint32(route.Status().Flags) &&
				existing.Attributes.OutIface == linkIndex && existing.Attributes.Priority == route.Status().Priority &&
				existing.Attributes.Table == uint32(route.Status().Table) {
				matchFound = true

				break
			}

			// delete route, it doesn't match the spec
			if err := conn.Route.Delete(existing); err != nil {
				return fmt.Errorf("error removing route: %w", err)
			}

			logger.Sugar().Infof("removed route to %s via %s (link %q)", destinationStr, route.Status().Gateway, route.Status().OutLinkName)
		}

		if matchFound {
			return nil
		}

		// add route
		msg := &rtnetlink.RouteMessage{
			Family:    uint8(route.Status().Family),
			DstLength: route.Status().Destination.Bits,
			Protocol:  uint8(route.Status().Protocol),
			Scope:     uint8(route.Status().Scope),
			Type:      uint8(route.Status().Type),
			Flags:     uint32(route.Status().Flags),
			Attributes: rtnetlink.RouteAttributes{
				Dst:      route.Status().Destination.IP.IPAddr().IP,
				Gateway:  route.Status().Gateway.IPAddr().IP,
				OutIface: linkIndex,
				Priority: route.Status().Priority,
				Table:    uint32(route.Status().Table),
			},
		}

		if err := conn.Route.Add(msg); err != nil {
			return fmt.Errorf("error adding route: %w, message %+v", err, *msg)
		}

		logger.Sugar().Infof("created route to %s via %s (link %q)", destinationStr, route.Status().Gateway, route.Status().OutLinkName)
	}

	return nil
}
