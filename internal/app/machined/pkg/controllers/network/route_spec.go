// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// RouteSpecController applies network.RouteSpec to the actual interfaces.
type RouteSpecController struct {
	// installedProtocols keeps the protocol of the route last installed for each spec.
	//
	// The spec might change the protocol in place (e.g. when the merge picks a route from another config layer for
	// the same key), and if the new version is not applied (yet), the route in the kernel still has the old protocol,
	// so it has to be removed on teardown as well.
	installedProtocols map[resource.ID]nethelpers.RouteProtocol
}

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
	watcher, err := watch.NewRtNetlink(trigger.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_LINK|unix.RTMGRP_IPV4_ROUTE|unix.RTMGRP_IPV6_ROUTE)
	if err != nil {
		return err
	}

	defer watcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	if ctrl.installedProtocols == nil {
		ctrl.installedProtocols = map[resource.ID]nethelpers.RouteProtocol{}
	}

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

func routePriorityMatches(actual uint32, expected *network.RouteSpecSpec) bool {
	if actual == expected.Priority {
		return true
	}

	// Linux assigns metric 1024 to IPv6 routes added without an explicit metric.
	return expected.Family == nethelpers.FamilyInet6 &&
		expected.Priority == 0 &&
		actual == network.DefaultRouteMetric
}

// RouteScopeMatches compares the actual rtm scope (as reported by the kernel), with the expected scope, as defined in the RouteSpec.
//
// The kernel accepts any scope on RTM_NEWROUTE. However, the route scope is an IPv4-only concept.
// In the case of IPv6, the kernel ignores the provided scope, and the IPv6 FIB (fib6_info) doesn't even have an equivalent scope field.
// When the route is read back, the kernel always fills the returned route's scope with RT_SCOPE_UNIVERSE (nethelpers.ScopeGlobal), in
// rt6_fill_node(). That's why we only assert the scope in non-IPv6 scenarios.
func RouteScopeMatches(actual uint8, expected *network.RouteSpecSpec) bool {
	if expected.Family == nethelpers.FamilyInet6 {
		return true
	}

	return actual == uint8(expected.Scope)
}

// RouteDestinationMatches reports whether an existing kernel route is a route to the expected destination.
//
// A default route (zero-length prefix) carries no RTA_DST in the kernel, so its Dst is nil; the
// expected destination may be either the zero Prefix (nil) or an explicit 0.0.0.0/0 (non-nil).
// The length match alone identifies it (a default is unique per family/table/priority).
func RouteDestinationMatches(route *rtnetlink.RouteMessage, destination netip.Prefix) bool {
	if int(route.DstLength) != netipPrefixBitsCorrected(destination) {
		return false
	}

	// the kernel reports the destination masked to the prefix length, so compare against the masked expected destination
	return route.DstLength == 0 || route.Attributes.Dst.Equal(destination.Masked().Addr().AsSlice())
}

// linkIndexMatches reports whether the egress link the kernel reports matches the one the spec asked for.
//
// A spec with no out-link (link index zero, e.g. a route learned from a numbered BGP peer) matches
// whatever egress device the kernel resolved from the gateway: the kernel always reports a resolved
// interface index back, so comparing it verbatim would never match and the route would be deleted
// and re-added on every reconcile.
func linkIndexMatches(actual, expected uint32) bool {
	return expected == 0 || actual == expected
}

// findRoutesByKey returns the existing kernel routes which have the same key as the spec.
//
// The kernel keeps a single route per table, family, destination and priority (see network.RouteID): a route
// with the same key but different next-hops can't be added next to it, it has to replace the existing one.
func findRoutesByKey(existingRoutes []rtnetlink.RouteMessage, expected *network.RouteSpecSpec) []*rtnetlink.RouteMessage {
	var result []*rtnetlink.RouteMessage //nolint:prealloc

	for i, route := range existingRoutes {
		if route.Family != uint8(expected.Family) {
			continue
		}

		// TOS (IPv4) and source-specific (IPv6) routes are part of a different key, and they are never created by Talos
		if route.Tos != 0 || route.SrcLength != 0 {
			continue
		}

		if !RouteDestinationMatches(&existingRoutes[i], expected.Destination) {
			continue
		}

		if nethelpers.RoutingTable(route.Table) != expected.Table {
			continue
		}

		if !routePriorityMatches(route.Attributes.Priority, expected) {
			continue
		}

		result = append(result, &existingRoutes[i])
	}

	return result
}

// findOwnedRoutesByKey returns the existing kernel routes with the same key as the spec which are managed by the spec.
//
// A route with the same key is owned by the spec if its protocol matches the spec's protocol, or the protocol the spec
// was last installed with (the spec protocol might have changed, e.g. a layer change). Routes with the same key
// created by someone else (e.g. by the kernel for a connected subnet, or learned from router advertisements) are never touched.
func findOwnedRoutesByKey(existingRoutes []rtnetlink.RouteMessage, expected *network.RouteSpecSpec, installedProtocol nethelpers.RouteProtocol, installed bool) []*rtnetlink.RouteMessage {
	return xslices.Filter(findRoutesByKey(existingRoutes, expected), func(existing *rtnetlink.RouteMessage) bool {
		return existing.Protocol == uint8(expected.Protocol) || (installed && existing.Protocol == uint8(installedProtocol))
	})
}

// routeMatchesSpec reports whether an existing kernel route (with the same key) fully matches the spec.
func routeMatchesSpec(existing *rtnetlink.RouteMessage, expected *network.RouteSpecSpec, linkIndex uint32, multipath []rtnetlink.NextHop) bool {
	return routeGatewayMatches(existing, expected) &&
		RouteScopeMatches(existing.Scope, expected) &&
		nethelpers.RouteFlags(existing.Flags).Equal(expected.Flags) &&
		existing.Protocol == uint8(expected.Protocol) &&
		linkIndexMatches(existing.Attributes.OutIface, linkIndex) &&
		(value.IsZero(expected.Source) || existing.Attributes.Src.Equal(expected.Source.AsSlice())) &&
		routeMTU(existing) == expected.MTU &&
		existing.Type == uint8(expected.Type) &&
		multipathEqual(existing.Attributes.Multipath, multipath)
}

func routeMTU(route *rtnetlink.RouteMessage) uint32 {
	if route.Attributes.Metrics == nil {
		return 0
	}

	return route.Attributes.Metrics.MTU
}

// routeSpecGatewayString returns the gateway of a route spec for logging, empty for gateway-less and multipath routes.
func routeSpecGatewayString(spec *network.RouteSpecSpec) string {
	if value.IsZero(spec.Gateway) {
		return ""
	}

	return spec.Gateway.String()
}

// routeGatewayString returns the gateway of an existing kernel route for logging, empty for multipath routes.
func routeGatewayString(route *rtnetlink.RouteMessage) string {
	if route.Attributes.Via != nil {
		return route.Attributes.Via.Addr.String()
	}

	if route.Attributes.Gateway != nil {
		return route.Attributes.Gateway.String()
	}

	return ""
}

// logRouteMismatch logs (at debug level) every attribute of the existing kernel route next to the one the spec asks for,
// to tell which of them made the route not match the spec.
//
// The fields are only built if debug logging is enabled.
func logRouteMismatch(logger *zap.Logger, existing *rtnetlink.RouteMessage, expected *network.RouteSpecSpec, destinationStr, gatewayStr, sourceStr string, linkIndex uint32) {
	if ce := logger.Check(zap.DebugLevel, "route mismatch"); ce != nil {
		ce.Write(routeMismatchFields(existing, expected, destinationStr, gatewayStr, sourceStr, linkIndex)...)
	}
}

// routeMismatchFields returns log fields describing the differences between an existing kernel route and the spec.
func routeMismatchFields(existing *rtnetlink.RouteMessage, expected *network.RouteSpecSpec, destinationStr, gatewayStr, sourceStr string, linkIndex uint32) []zap.Field {
	return []zap.Field{
		zap.String("destination", destinationStr),
		zap.Stringer("table", expected.Table),
		zap.String("link", expected.OutLinkName),
		zap.Uint32("priority", expected.Priority),
		zap.Stringer("family", expected.Family),
		zap.String("old_gateway", routeGatewayString(existing)),
		zap.String("new_gateway", gatewayStr),
		zap.Int("old_next_hops", len(existing.Attributes.Multipath)),
		zap.Int("new_next_hops", len(expected.NextHops)),
		zap.Stringer("old_scope", nethelpers.Scope(existing.Scope)),
		zap.Stringer("new_scope", expected.Scope),
		zap.Stringer("old_flags", nethelpers.RouteFlags(existing.Flags)),
		zap.Stringer("new_flags", expected.Flags),
		zap.Stringer("old_protocol", nethelpers.RouteProtocol(existing.Protocol)),
		zap.Stringer("new_protocol", expected.Protocol),
		zap.Uint32("old_link_index", existing.Attributes.OutIface),
		zap.Uint32("new_link_index", linkIndex),
		zap.Stringer("old_source", existing.Attributes.Src),
		zap.String("new_source", sourceStr),
		zap.Uint32("old_mtu", routeMTU(existing)),
		zap.Uint32("new_mtu", expected.MTU),
		zap.Stringer("old_type", nethelpers.RouteType(existing.Type)),
		zap.Stringer("new_type", expected.Type),
	}
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
// The next-hops are installed in the canonical order (see network.NormalizeNextHops), whatever order the
// spec lists them in, so that the resulting kernel route doesn't depend on the producer.
//
// It returns false if any next-hop link cannot be resolved yet, in which case the route should be
// retried once the link appears.
func buildMultipath(family nethelpers.Family, links []rtnetlink.LinkMessage, nextHops []network.RouteNextHop) ([]rtnetlink.NextHop, bool) {
	result := make([]rtnetlink.NextHop, 0, len(nextHops))

	for _, nh := range network.NormalizeNextHops(slices.Clone(nextHops)) {
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
				Hops:    uint8(weight - 1), // the kernel encodes the next-hop weight as (weight - 1)
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
//
// The comparison is order-insensitive: the kernel keeps the next-hops in the order they were installed,
// which is not necessarily the order the spec lists them in. Each expected next-hop has to be matched
// by a distinct existing one, so the sets have to be equal as multisets.
//
// An expected next-hop with no out-link matches whatever egress device the kernel resolved from the
// gateway (see linkIndexMatches), mirroring the single-gateway path in syncRoute.
func multipathEqual(existing, expected []rtnetlink.NextHop) bool {
	if len(existing) != len(expected) {
		return false
	}

	matched := make([]bool, len(existing))

	for _, want := range expected {
		found := false

		for i := range existing {
			if matched[i] || !nextHopEqual(existing[i], want) {
				continue
			}

			matched[i] = true
			found = true

			break
		}

		if !found {
			return false
		}
	}

	return true
}

// nextHopEqual reports whether an existing kernel next-hop matches the expected one.
func nextHopEqual(existing, expected rtnetlink.NextHop) bool {
	return linkIndexMatches(existing.Hop.IfIndex, expected.Hop.IfIndex) &&
		existing.Hop.Hops == expected.Hop.Hops &&
		existing.Gateway.Equal(expected.Gateway) &&
		viaEqual(existing.Via, expected.Via)
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

	installedProtocol, installed := ctrl.installedProtocols[route.Metadata().ID()]

	switch route.Metadata().Phase() {
	case resource.PhaseTearingDown:
		// the route with the same key is owned by this spec, even if it is not up to date with the latest version
		// of the spec, but don't touch routes created by someone else (e.g. by the kernel)
		for _, existing := range findOwnedRoutesByKey(routes, route.TypedSpec(), installedProtocol, installed) {
			// delete route
			if err := conn.Route.Delete(existing); err != nil {
				return fmt.Errorf("error removing route: %w", err)
			}

			logger.Info(
				"deleted route",
				zap.String("destination", destinationStr),
				zap.String("gateway", routeGatewayString(existing)),
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

		delete(ctrl.installedProtocols, route.Metadata().ID())
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

		// only the routes owned by this spec are considered: a route with the same key created by someone else
		// (e.g. by the kernel) is left alone, and adding the route next to it is up to the kernel (it might fail)
		existingRoutes := findOwnedRoutesByKey(routes, route.TypedSpec(), installedProtocol, installed)

		// check if an existing route matches the spec: if it does, skip update
		matchIdx := slices.IndexFunc(existingRoutes, func(existing *rtnetlink.RouteMessage) bool {
			return routeMatchesSpec(existing, route.TypedSpec(), linkIndex, multipath)
		})

		// if there's no match, the first route with the same key is replaced in place, so that the route is never
		// missing in the kernel while it's being updated (e.g. a gateway change, or a change to/from multipath)
		var replaced *rtnetlink.RouteMessage

		if matchIdx == -1 && len(existingRoutes) > 0 {
			replaced = existingRoutes[0]
		}

		for i, existing := range existingRoutes {
			if i == matchIdx || existing == replaced {
				continue
			}

			// delete the route, it doesn't match the spec, and it can't be replaced as there's another route with the same key
			if err := conn.Route.Delete(existing); err != nil {
				return fmt.Errorf("error removing route: %w", err)
			}

			logger.Info(
				"deleted route due to mismatch",
				zap.String("destination", destinationStr),
				zap.String("gateway", routeGatewayString(existing)),
				zap.Int("next_hops", len(existing.Attributes.Multipath)),
				zap.Stringer("table", route.TypedSpec().Table),
				zap.String("link", route.TypedSpec().OutLinkName),
				zap.Uint32("priority", route.TypedSpec().Priority),
				zap.Stringer("family", route.TypedSpec().Family),
				zap.Stringer("type", route.TypedSpec().Type),
			)

			logRouteMismatch(logger, existing, route.TypedSpec(), destinationStr, gatewayStr, sourceStr, linkIndex)
		}

		if matchIdx != -1 {
			ctrl.installedProtocols[route.Metadata().ID()] = route.TypedSpec().Protocol

			return nil
		}

		routeAttributes := rtnetlink.RouteAttributes{
			Dst:      route.TypedSpec().Destination.Masked().Addr().AsSlice(),
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

		if replaced != nil {
			if err := conn.Route.Replace(msg); err != nil {
				return fmt.Errorf("error replacing route: %w, message %+v", err, *msg)
			}

			ctrl.installedProtocols[route.Metadata().ID()] = route.TypedSpec().Protocol

			logger.Info(
				"replaced route",
				zap.String("destination", destinationStr),
				zap.String("gateway", gatewayStr),
				zap.String("old_gateway", routeGatewayString(replaced)),
				zap.Int("next_hops", len(route.TypedSpec().NextHops)),
				zap.Int("old_next_hops", len(replaced.Attributes.Multipath)),
				zap.Stringer("table", route.TypedSpec().Table),
				zap.String("link", route.TypedSpec().OutLinkName),
				zap.Uint32("priority", route.TypedSpec().Priority),
				zap.Stringer("family", route.TypedSpec().Family),
				zap.Stringer("type", route.TypedSpec().Type),
			)

			logRouteMismatch(logger, replaced, route.TypedSpec(), destinationStr, gatewayStr, sourceStr, linkIndex)

			return nil
		}

		if err := conn.Route.Add(msg); err != nil {
			return fmt.Errorf("error adding route: %w, message %+v", err, *msg)
		}

		ctrl.installedProtocols[route.Metadata().ID()] = route.TypedSpec().Protocol

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
