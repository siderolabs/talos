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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// RouteConfigController manages network.RouteSpec based on machine configuration, kernel cmdline.
type RouteConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *RouteConfigController) Name() string {
	return "network.RouteConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RouteConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.DeviceConfigSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RouteConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *RouteConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		touchedIDs := make(map[resource.ID]struct{})

		items, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.DeviceConfigSpecType, "", resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		}

		ignoredInterfaces := map[string]struct{}{}

		devices := make([]talosconfig.Device, len(items.Items))

		for i, item := range items.Items {
			device := item.(*network.DeviceConfigSpec).TypedSpec().Device

			devices[i] = device

			if device.Ignore() {
				ignoredInterfaces[device.Interface()] = struct{}{}
			}
		}

		if len(devices) > 0 {
			for _, device := range devices {
				if device.Ignore() {
					ignoredInterfaces[device.Interface()] = struct{}{}
				}
			}
		}

		// parse kernel cmdline for the default gateway
		cmdlineRoutes := ctrl.parseCmdline(logger)
		for _, cmdlineRoute := range cmdlineRoutes {
			if _, ignored := ignoredInterfaces[cmdlineRoute.OutLinkName]; !ignored {
				var ids []string

				ids, err = ctrl.apply(ctx, r, []network.RouteSpecSpec{cmdlineRoute})
				if err != nil {
					return fmt.Errorf("error applying cmdline route: %w", err)
				}

				for _, id := range ids {
					touchedIDs[id] = struct{}{}
				}
			}
		}

		// parse machine configuration for static routes
		if len(devices) > 0 {
			addresses := ctrl.processDevicesConfiguration(logger, devices)

			var ids []string

			ids, err = ctrl.apply(ctx, r, addresses)
			if err != nil {
				return fmt.Errorf("error applying machine configuration address: %w", err)
			}

			for _, id := range ids {
				touchedIDs[id] = struct{}{}
			}
		}

		// list routes for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.RouteSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				// skip specs created by other controllers
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up routes: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *RouteConfigController) apply(ctx context.Context, r controller.Runtime, routes []network.RouteSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(routes))

	for _, route := range routes {
		id := network.LayeredID(route.ConfigLayer, network.RouteID(route.Table, route.Family, route.Destination, route.Gateway, route.Priority, route.OutLinkName))

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewRouteSpec(network.ConfigNamespaceName, id),
			func(r *network.RouteSpec) error {
				*r.TypedSpec() = route

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *RouteConfigController) parseCmdline(logger *zap.Logger) (routes []network.RouteSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Info("ignoring error", zap.Error(err))

		return
	}

	for idx, linkConfig := range settings.LinkConfigs {
		if value.IsZero(linkConfig.Gateway) {
			continue
		}

		// add a default gateway route
		defaultGatewayRoute := network.RouteSpecSpec{
			Gateway:     linkConfig.Gateway,
			Scope:       nethelpers.ScopeGlobal,
			Table:       nethelpers.TableMain,
			Priority:    network.DefaultRouteMetric + uint32(idx), // set different priorities to avoid a conflict
			Protocol:    nethelpers.ProtocolBoot,
			Type:        nethelpers.TypeUnicast,
			OutLinkName: linkConfig.LinkName,
			ConfigLayer: network.ConfigCmdline,
		}

		if defaultGatewayRoute.Gateway.Is6() {
			defaultGatewayRoute.Family = nethelpers.FamilyInet6
		} else {
			defaultGatewayRoute.Family = nethelpers.FamilyInet4
		}

		defaultGatewayRoute.Normalize()

		routes = append(routes, defaultGatewayRoute)

		// for IPv4, if the gateway is not directly reachable on the link, add a link-scope route for the gateway
		if linkConfig.Gateway.Is4() && !linkConfig.Address.Contains(linkConfig.Gateway) {
			routes = append(routes, network.RouteSpecSpec{
				Family:      nethelpers.FamilyInet4,
				Destination: netip.PrefixFrom(linkConfig.Gateway, linkConfig.Gateway.BitLen()),
				Source:      linkConfig.Address.Addr(),
				OutLinkName: linkConfig.LinkName,
				Table:       nethelpers.TableMain,
				Priority:    defaultGatewayRoute.Priority,
				Scope:       nethelpers.ScopeLink,
				Type:        nethelpers.TypeUnicast,
				Protocol:    nethelpers.ProtocolBoot,
				ConfigLayer: network.ConfigCmdline,
			})
		}
	}

	return routes
}

//nolint:gocyclo,cyclop
func (ctrl *RouteConfigController) processDevicesConfiguration(logger *zap.Logger, devices []talosconfig.Device) (routes []network.RouteSpecSpec) {
	convert := func(linkName string, in talosconfig.Route) (route network.RouteSpecSpec, err error) {
		if in.Network() != "" {
			route.Destination, err = netip.ParsePrefix(in.Network())
			if err != nil {
				return route, fmt.Errorf("error parsing route network: %w", err)
			}
		}

		if in.Gateway() != "" {
			route.Gateway, err = netip.ParseAddr(in.Gateway())
			if err != nil {
				return route, fmt.Errorf("error parsing route gateway: %w", err)
			}
		}

		if in.Source() != "" {
			route.Source, err = netip.ParseAddr(in.Source())
			if err != nil {
				return route, fmt.Errorf("error parsing route source: %w", err)
			}
		}

		normalizedFamily := route.Normalize()

		route.Priority = in.Metric()
		if route.Priority == 0 {
			route.Priority = network.DefaultRouteMetric
		}

		route.MTU = in.MTU()

		switch {
		case !value.IsZero(route.Gateway) && route.Gateway.Is6():
			route.Family = nethelpers.FamilyInet6
		case !value.IsZero(route.Destination) && route.Destination.Addr().Is6():
			route.Family = nethelpers.FamilyInet6
		case normalizedFamily != 0:
			route.Family = normalizedFamily
		default:
			route.Family = nethelpers.FamilyInet4
		}

		route.Table = nethelpers.TableMain
		route.Protocol = nethelpers.ProtocolStatic
		route.OutLinkName = linkName
		route.ConfigLayer = network.ConfigMachineConfiguration

		route.Type = nethelpers.TypeUnicast

		if route.Destination.Addr().IsMulticast() {
			route.Type = nethelpers.TypeMulticast
		}

		return route, nil
	}

	for _, device := range devices {
		if device.Ignore() {
			continue
		}

		for _, route := range device.Routes() {
			routeSpec, err := convert(device.Interface(), route)
			if err != nil {
				logger.Sugar().Infof("skipping route %q -> %q on interface %q: %s", route.Network(), route.Gateway(), device.Interface(), err)

				continue
			}

			routes = append(routes, routeSpec)
		}

		for _, vlan := range device.Vlans() {
			vlanLinkName := nethelpers.VLANLinkName(device.Interface(), vlan.ID())

			for _, route := range vlan.Routes() {
				routeSpec, err := convert(vlanLinkName, route)
				if err != nil {
					logger.Sugar().Infof("skipping route %q -> %q on interface %q: %s", route.Network(), route.Gateway(), vlanLinkName, err)

					continue
				}

				routes = append(routes, routeSpec)
			}
		}
	}

	return routes
}
