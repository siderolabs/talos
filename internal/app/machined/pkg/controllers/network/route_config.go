// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"inet.af/netaddr"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
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

		var cfgProvider talosconfig.Provider

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.(*config.MachineConfig).Config()
		}

		ignoredInterfaces := map[string]struct{}{}

		if cfgProvider != nil {
			for _, device := range cfgProvider.Machine().Network().Devices() {
				if device.Ignore() {
					ignoredInterfaces[device.Interface()] = struct{}{}
				}
			}
		}

		// parse kernel cmdline for the default gateway
		cmdlineRoute := ctrl.parseCmdline(logger)
		if !cmdlineRoute.Gateway.IsZero() {
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
		if cfgProvider != nil {
			addresses := ctrl.parseMachineConfiguration(logger, cfgProvider)

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
	}
}

func (ctrl *RouteConfigController) apply(ctx context.Context, r controller.Runtime, routes []network.RouteSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(routes))

	for _, route := range routes {
		route := route
		id := network.LayeredID(route.ConfigLayer, network.RouteID(route.Table, route.Family, route.Destination, route.Gateway, route.Priority))

		if err := r.Modify(
			ctx,
			network.NewRouteSpec(network.ConfigNamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.RouteSpec).TypedSpec() = route

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}

func (ctrl *RouteConfigController) parseCmdline(logger *zap.Logger) (route network.RouteSpecSpec) {
	if ctrl.Cmdline == nil {
		return
	}

	settings, err := ParseCmdlineNetwork(ctrl.Cmdline)
	if err != nil {
		logger.Info("ignoring error", zap.Error(err))

		return
	}

	if settings.Gateway.IsZero() {
		return
	}

	route.Gateway = settings.Gateway

	if route.Gateway.Is6() {
		route.Family = nethelpers.FamilyInet6
	} else {
		route.Family = nethelpers.FamilyInet4
	}

	route.Scope = nethelpers.ScopeGlobal
	route.Table = nethelpers.TableMain
	route.Priority = DefaultRouteMetric
	route.Protocol = nethelpers.ProtocolBoot
	route.Type = nethelpers.TypeUnicast
	route.OutLinkName = settings.LinkName
	route.ConfigLayer = network.ConfigCmdline

	route.Normalize()

	return route
}

//nolint:gocyclo,cyclop
func (ctrl *RouteConfigController) parseMachineConfiguration(logger *zap.Logger, cfgProvider talosconfig.Provider) (routes []network.RouteSpecSpec) {
	convert := func(linkName string, in talosconfig.Route) (route network.RouteSpecSpec, err error) {
		if in.Network() != "" {
			route.Destination, err = netaddr.ParseIPPrefix(in.Network())
			if err != nil {
				return route, fmt.Errorf("error parsing route network: %w", err)
			}
		}

		if in.Gateway() != "" {
			route.Gateway, err = netaddr.ParseIP(in.Gateway())
			if err != nil {
				return route, fmt.Errorf("error parsing route gateway: %w", err)
			}
		}

		if in.Source() != "" {
			route.Source, err = netaddr.ParseIP(in.Source())
			if err != nil {
				return route, fmt.Errorf("error parsing route source: %w", err)
			}
		}

		route.Normalize()

		route.Priority = in.Metric()
		if route.Priority == 0 {
			route.Priority = DefaultRouteMetric
		}

		switch {
		case !route.Gateway.IsZero() && route.Gateway.Is6():
			route.Family = nethelpers.FamilyInet6
		case !route.Destination.IsZero() && route.Destination.IP().Is6():
			route.Family = nethelpers.FamilyInet6
		default:
			route.Family = nethelpers.FamilyInet4
		}

		route.Table = nethelpers.TableMain
		route.Protocol = nethelpers.ProtocolStatic
		route.OutLinkName = linkName
		route.ConfigLayer = network.ConfigMachineConfiguration

		route.Type = nethelpers.TypeUnicast

		if route.Destination.IP().IsMulticast() {
			route.Type = nethelpers.TypeMulticast
		}

		return route, nil
	}

	for _, device := range cfgProvider.Machine().Network().Devices() {
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
			vlanLinkName := fmt.Sprintf("%s.%d", device.Interface(), vlan.ID())

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
