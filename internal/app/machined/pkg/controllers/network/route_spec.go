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
	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/gen/value"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
//nolint:gocyclo
func (ctrl *RouteSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// watch link changes as some routes might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(trigger.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_LINK|unix.RTMGRP_IPV4_ROUTE)
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
		list, err := safe.ReaderListAll[*network.RouteSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing source network routes: %w", err)
		}

		// add finalizers for all live resources
		for res := range list.All() {
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

		var multiErr *multierror.Error

		// loop over routes and make reconcile decision
		for route := range list.All() {
			if err = ctrl.syncRoute(ctx, r, logger, conn, links, routes, route); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
		}

		if err = multiErr.ErrorOrNil(); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

// netipPrefixBitsCorrected returns the number of bits in the prefix, corrected for zero value to have bits of 0.
//
// Go stdlib returns -1 for zero value, which is not what we want.
func netipPrefixBitsCorrected(p netip.Prefix) int {
	if p.Addr().AsSlice() == nil {
		return 0
	}

	return p.Bits()
}

func findMatchingRoutes(existingRoutes []rtnetlink.RouteMessage, expected *network.RouteSpecSpec) []*rtnetlink.RouteMessage {
	var result []*rtnetlink.RouteMessage //nolint:prealloc

	for i, route := range existingRoutes {
		if route.Family != uint8(expected.Family) {
			continue
		}

		if int(route.DstLength) != netipPrefixBitsCorrected(expected.Destination) {
			continue
		}

		// a default route (zero-length prefix) carries no RTA_DST in the kernel, so its Dst is nil; the
		// expected destination may be either the zero Prefix (nil) or an explicit 0.0.0.0/0 (non-nil).
		// The length match alone identifies it (a default is unique per family/table/priority).
		if route.DstLength != 0 && !route.Attributes.Dst.Equal(expected.Destination.Addr().AsSlice()) {
			continue
		}

		if !routeGatewayMatches(&existingRoutes[i], expected) {
			continue
		}

		if nethelpers.RoutingTable(route.Table) != expected.Table {
			continue
		}

		if route.Attributes.Priority != expected.Priority {
			continue
		}

		result = append(result, &existingRoutes[i])
	}

	return result
}

// crossFamilyVia returns an RTA_VIA next-hop when the gateway's address family differs from the
// route's destination family (RFC 8950, e.g. an IPv4 route via an IPv6 link-local next-hop).
// It returns nil for a same-family (or absent) gateway, in which case RTA_GATEWAY is used.
func crossFamilyVia(family nethelpers.Family, gw netip.Addr) *rtnetlink.RouteVia {
	if !gw.IsValid() {
		return nil
	}

	gwIsV6 := gw.Is6() && !gw.Is4In6()

	switch family {
	case nethelpers.FamilyInet4:
		if gwIsV6 {
			return &rtnetlink.RouteVia{Family: unix.AF_INET6, Addr: gw.AsSlice()}
		}
	case nethelpers.FamilyInet6:
		if !gwIsV6 {
			return &rtnetlink.RouteVia{Family: unix.AF_INET, Addr: gw.AsSlice()}
		}
	}

	return nil
}

func viaEqual(a, b *rtnetlink.RouteVia) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Family == b.Family && a.Addr.Equal(b.Addr)
}

// routeGatewayMatches reports whether an existing kernel route's next-hop matches the spec, handling
// the single-gateway, cross-family (RTA_VIA), and multipath cases.
func routeGatewayMatches(existing *rtnetlink.RouteMessage, expected *network.RouteSpecSpec) bool {
	if len(expected.NextHops) > 0 {
		// multipath: top-level gateway/via are empty; per-hop comparison happens in multipathEqual
		return existing.Attributes.Gateway == nil && existing.Attributes.Via == nil
	}

	if via := crossFamilyVia(expected.Family, expected.Gateway); via != nil {
		return viaEqual(existing.Attributes.Via, via)
	}

	return existing.Attributes.Gateway.Equal(expected.Gateway.AsSlice())
}

// buildMultipath builds rtnetlink multipath next-hops from the spec, resolving link names to indices.
//
// It returns false if any next-hop link cannot be resolved yet, in which case the route should be
// retried once the link appears.
func buildMultipath(family nethelpers.Family, links []rtnetlink.LinkMessage, nextHops []network.RouteNextHop) ([]rtnetlink.NextHop, bool) {
	result := make([]rtnetlink.NextHop, 0, len(nextHops))

	for _, nh := range nextHops {
		ifIndex := resolveLinkName(links, nh.OutLinkName)
		if ifIndex == 0 && nh.OutLinkName != "" {
			return nil, false
		}

		weight := nh.Weight
		if weight == 0 {
			weight = 1
		}

		hop := rtnetlink.NextHop{
			Hop: rtnetlink.RTNextHop{
				IfIndex: ifIndex,
				Hops:    weight - 1, // the kernel encodes the next-hop weight as (weight - 1)
			},
		}

		if via := crossFamilyVia(family, nh.Gateway); via != nil {
			hop.Via = via
		} else {
			hop.Gateway = nh.Gateway.AsSlice()
		}

		result = append(result, hop)
	}

	return result, true
}

// multipathEqual reports whether an existing kernel multipath set matches the expected next-hops.
func multipathEqual(existing, expected []rtnetlink.NextHop) bool {
	if len(existing) != len(expected) {
		return false
	}

	for i := range expected {
		if existing[i].Hop.IfIndex != expected[i].Hop.IfIndex ||
			existing[i].Hop.Hops != expected[i].Hop.Hops ||
			!existing[i].Gateway.Equal(expected[i].Gateway) ||
			!viaEqual(existing[i].Via, expected[i].Via) {
			return false
		}
	}

	return true
}

//nolint:gocyclo,cyclop
func (ctrl *RouteSpecController) syncRoute(ctx context.Context, r controller.Runtime, logger *zap.Logger, conn *rtnetlink.Conn,
	links []rtnetlink.LinkMessage, routes []rtnetlink.RouteMessage, route *network.RouteSpec,
) error {
	linkIndex := resolveLinkName(links, route.TypedSpec().OutLinkName)

	isMultipath := len(route.TypedSpec().NextHops) > 0

	destinationStr := route.TypedSpec().Destination.String()
	if value.IsZero(route.TypedSpec().Destination) {
		destinationStr = "default"
	}

	sourceStr := route.TypedSpec().Source.String()
	if value.IsZero(route.TypedSpec().Source) {
		sourceStr = ""
	}

	gatewayStr := route.TypedSpec().Gateway.String()
	if value.IsZero(route.TypedSpec().Gateway) {
		gatewayStr = ""
	}

	switch route.Metadata().Phase() {
	case resource.PhaseTearingDown:
		for _, existing := range findMatchingRoutes(routes, route.TypedSpec()) {
			// delete route
			if err := conn.Route.Delete(existing); err != nil {
				return fmt.Errorf("error removing route: %w", err)
			}

			logger.Info(
				"deleted route",
				zap.String("destination", destinationStr),
				zap.String("gateway", gatewayStr),
				zap.Stringer("table", route.TypedSpec().Table),
				zap.String("link", route.TypedSpec().OutLinkName),
				zap.Uint32("priority", route.TypedSpec().Priority),
				zap.Stringer("family", route.TypedSpec().Family),
				zap.Stringer("type", route.TypedSpec().Type),
			)
		}

		// now remove finalizer as address was deleted
		if err := r.RemoveFinalizer(ctx, route.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer: %w", err)
		}
	case resource.PhaseRunning:
		if linkIndex == 0 && route.TypedSpec().OutLinkName != "" {
			// route can't be created as link doesn't exist (yet), skip it
			return nil
		}

		var multipath []rtnetlink.NextHop

		if isMultipath {
			var ok bool

			if multipath, ok = buildMultipath(route.TypedSpec().Family, links, route.TypedSpec().NextHops); !ok {
				// route can't be created as a next-hop link doesn't exist (yet), skip it
				return nil
			}
		}

		matchFound := false

		for _, existing := range findMatchingRoutes(routes, route.TypedSpec()) {
			var existingMTU uint32

			if existing.Attributes.Metrics != nil {
				existingMTU = existing.Attributes.Metrics.MTU
			}

			// check if existing route matches the spec: if it does, skip update
			if existing.Scope == uint8(route.TypedSpec().Scope) && nethelpers.RouteFlags(existing.Flags).Equal(route.TypedSpec().Flags) &&
				existing.Protocol == uint8(route.TypedSpec().Protocol) &&
				// when no out-link is requested, accept whatever egress device the kernel resolved
				(linkIndex == 0 || existing.Attributes.OutIface == linkIndex) &&
				(value.IsZero(route.TypedSpec().Source) ||
					existing.Attributes.Src.Equal(route.TypedSpec().Source.AsSlice())) &&
				existingMTU == route.TypedSpec().MTU &&
				existing.Type == uint8(route.TypedSpec().Type) &&
				multipathEqual(existing.Attributes.Multipath, multipath) {
				matchFound = true

				continue
			}

			// delete the route, it doesn't match the spec
			if err := conn.Route.Delete(existing); err != nil {
				return fmt.Errorf("error removing route: %w", err)
			}

			logger.Debug(
				"removed route due to mismatch",
				zap.String("destination", destinationStr),
				zap.String("gateway", gatewayStr),
				zap.Stringer("table", route.TypedSpec().Table),
				zap.String("link", route.TypedSpec().OutLinkName),
				zap.Uint32("priority", route.TypedSpec().Priority),
				zap.Stringer("family", route.TypedSpec().Family),
				zap.Stringer("old_scope", nethelpers.Scope(existing.Scope)),
				zap.Stringer("new_scope", route.TypedSpec().Scope),
				zap.Stringer("old_flags", nethelpers.RouteFlags(existing.Flags)),
				zap.Stringer("new_flags", route.TypedSpec().Flags),
				zap.Stringer("old_protocol", nethelpers.RouteProtocol(existing.Protocol)),
				zap.Stringer("new_protocol", route.TypedSpec().Protocol),
				zap.Uint32("old_link_index", existing.Attributes.OutIface),
				zap.Uint32("new_link_index", linkIndex),
				zap.Stringer("old_source", existing.Attributes.Src),
				zap.String("new_source", sourceStr),
				zap.Uint32("old_mtu", existingMTU),
				zap.Uint32("new_mtu", route.TypedSpec().MTU),
				zap.Stringer("old_type", nethelpers.RouteType(existing.Type)),
				zap.Stringer("new_type", route.TypedSpec().Type),
			)
		}

		if matchFound {
			return nil
		}

		routeAttributes := rtnetlink.RouteAttributes{
			Dst:      route.TypedSpec().Destination.Addr().AsSlice(),
			Src:      route.TypedSpec().Source.AsSlice(),
			Priority: route.TypedSpec().Priority,
			Table:    uint32(route.TypedSpec().Table),
		}

		if isMultipath {
			// multipath (ECMP): next-hops live in RTA_MULTIPATH, top-level gateway/oif stay unset
			routeAttributes.Multipath = multipath
		} else {
			routeAttributes.OutIface = linkIndex

			if via := crossFamilyVia(route.TypedSpec().Family, route.TypedSpec().Gateway); via != nil {
				// cross-family next-hop (RFC 8950): IPv4 dst via IPv6 link-local, etc.
				routeAttributes.Via = via
			} else {
				routeAttributes.Gateway = route.TypedSpec().Gateway.AsSlice()
			}
		}

		if route.TypedSpec().MTU != 0 {
			routeAttributes.Metrics = &rtnetlink.RouteMetrics{
				MTU: route.TypedSpec().MTU,
			}
		}

		// add route
		msg := &rtnetlink.RouteMessage{
			Family:     uint8(route.TypedSpec().Family),
			DstLength:  uint8(netipPrefixBitsCorrected(route.TypedSpec().Destination)),
			SrcLength:  0,
			Protocol:   uint8(route.TypedSpec().Protocol),
			Scope:      uint8(route.TypedSpec().Scope),
			Type:       uint8(route.TypedSpec().Type),
			Flags:      uint32(route.TypedSpec().Flags),
			Attributes: routeAttributes,
		}

		if err := conn.Route.Add(msg); err != nil {
			return fmt.Errorf("error adding route: %w, message %+v", err, *msg)
		}

		logger.Info(
			"created route",
			zap.String("destination", destinationStr),
			zap.String("gateway", gatewayStr),
			zap.Stringer("table", route.TypedSpec().Table),
			zap.String("link", route.TypedSpec().OutLinkName),
			zap.Uint32("priority", route.TypedSpec().Priority),
			zap.Stringer("family", route.TypedSpec().Family),
			zap.Stringer("type", route.TypedSpec().Type),
		)
	}

	return nil
}
