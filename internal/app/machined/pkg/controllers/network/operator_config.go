// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// OperatorConfigController manages network.OperatorSpec based on machine configuration, kernel cmdline.
type OperatorConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *OperatorConfigController) Name() string {
	return "network.OperatorConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *OperatorConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.DeviceConfigSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.ConfigNamespaceName,
			Type:      network.LinkSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *OperatorConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.OperatorSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *OperatorConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

		var (
			specs      []network.OperatorSpecSpec
			specErrors *multierror.Error
		)

		ignoredInterfaces := map[string]struct{}{}

		if ctrl.Cmdline != nil {
			var settings CmdlineNetworking

			settings, err = ParseCmdlineNetwork(ctrl.Cmdline)
			if err != nil {
				logger.Warn("ignored cmdline parse failure", zap.Error(err))
			}

			for _, link := range settings.IgnoreInterfaces {
				ignoredInterfaces[link] = struct{}{}
			}

			for _, linkConfig := range settings.LinkConfigs {
				if !linkConfig.DHCP {
					continue
				}

				specs = append(specs, network.OperatorSpecSpec{
					Operator:  network.OperatorDHCP4,
					LinkName:  linkConfig.LinkName,
					RequireUp: true,
					DHCP4: network.DHCP4OperatorSpec{
						RouteMetric: network.DefaultRouteMetric,
					},
					ConfigLayer: network.ConfigCmdline,
				})
			}
		}

		devices := xslices.Map(items.Items, func(item resource.Resource) talosconfig.Device {
			return item.(*network.DeviceConfigSpec).TypedSpec().Device
		})

		// operators from the config
		if len(devices) > 0 {
			for _, device := range devices {
				if device.Ignore() {
					ignoredInterfaces[device.Interface()] = struct{}{}
				}

				if _, ignore := ignoredInterfaces[device.Interface()]; ignore {
					continue
				}

				if device.DHCP() && device.DHCPOptions().IPv4() {
					routeMetric := device.DHCPOptions().RouteMetric()
					if routeMetric == 0 {
						routeMetric = network.DefaultRouteMetric
					}

					specs = append(specs, network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP4,
						LinkName:  device.Interface(),
						RequireUp: true,
						DHCP4: network.DHCP4OperatorSpec{
							RouteMetric: routeMetric,
						},
						ConfigLayer: network.ConfigMachineConfiguration,
					})
				}

				if device.DHCP() && device.DHCPOptions().IPv6() {
					routeMetric := device.DHCPOptions().RouteMetric()
					if routeMetric == 0 {
						routeMetric = network.DefaultRouteMetric
					}

					specs = append(specs, network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP6,
						LinkName:  device.Interface(),
						RequireUp: true,
						DHCP6: network.DHCP6OperatorSpec{
							RouteMetric: routeMetric,
							DUID:        device.DHCPOptions().DUIDv6(),
						},
						ConfigLayer: network.ConfigMachineConfiguration,
					})
				}

				for _, vlan := range device.Vlans() {
					if vlan.DHCP() && vlan.DHCPOptions().IPv4() {
						routeMetric := vlan.DHCPOptions().RouteMetric()
						if routeMetric == 0 {
							routeMetric = network.DefaultRouteMetric
						}

						specs = append(specs, network.OperatorSpecSpec{
							Operator:  network.OperatorDHCP4,
							LinkName:  nethelpers.VLANLinkName(device.Interface(), vlan.ID()),
							RequireUp: true,
							DHCP4: network.DHCP4OperatorSpec{
								RouteMetric: routeMetric,
							},
							ConfigLayer: network.ConfigMachineConfiguration,
						})
					}

					if vlan.DHCP() && vlan.DHCPOptions().IPv6() {
						routeMetric := vlan.DHCPOptions().RouteMetric()
						if routeMetric == 0 {
							routeMetric = network.DefaultRouteMetric
						}

						specs = append(specs, network.OperatorSpecSpec{
							Operator:  network.OperatorDHCP6,
							LinkName:  nethelpers.VLANLinkName(device.Interface(), vlan.ID()),
							RequireUp: true,
							DHCP6: network.DHCP6OperatorSpec{
								RouteMetric: routeMetric,
								DUID:        vlan.DHCPOptions().DUIDv6(),
							},
							ConfigLayer: network.ConfigMachineConfiguration,
						})
					}
				}
			}
		}

		// build configuredInterfaces from linkSpecs in `network-config` namespace
		// any link which has any configuration derived from the machine configuration or platform configuration should be ignored
		configuredInterfaces := map[string]struct{}{}

		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.LinkSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing link specs: %w", err)
		}

		for _, item := range list.Items {
			linkSpec := item.(*network.LinkSpec).TypedSpec()

			switch linkSpec.ConfigLayer {
			case network.ConfigDefault:
				// ignore default link specs
			case network.ConfigOperator:
				// specs produced by operators, ignore
			case network.ConfigCmdline, network.ConfigMachineConfiguration, network.ConfigPlatform:
				// interface is configured explicitly, don't run default dhcp4
				configuredInterfaces[linkSpec.Name] = struct{}{}
			}
		}

		// operators from defaults
		list, err = r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing link statuses: %w", err)
		}

		for _, item := range list.Items {
			linkStatus := item.(*network.LinkStatus) //nolint:errcheck,forcetypeassert

			if linkStatus.TypedSpec().Physical() {
				if _, configured := configuredInterfaces[linkStatus.Metadata().ID()]; !configured {
					if _, ignored := ignoredInterfaces[linkStatus.Metadata().ID()]; !ignored {
						// enable DHCPv4 operator on physical interfaces which don't have any explicit configuration and are not ignored
						specs = append(specs, network.OperatorSpecSpec{
							Operator:  network.OperatorDHCP4,
							LinkName:  linkStatus.Metadata().ID(),
							RequireUp: true,
							DHCP4: network.DHCP4OperatorSpec{
								RouteMetric: network.DefaultRouteMetric,
							},
							ConfigLayer: network.ConfigDefault,
						})
					}
				}
			}
		}

		var ids []string

		ids, err = ctrl.apply(ctx, r, specs)
		if err != nil {
			return fmt.Errorf("error applying operator specs: %w", err)
		}

		for _, id := range ids {
			touchedIDs[id] = struct{}{}
		}

		// list specs for cleanup
		list, err = r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
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

		// last, check if some specs failed to build; fail last so that other operator specs are applied successfully
		if err = specErrors.ErrorOrNil(); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:dupl
func (ctrl *OperatorConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.OperatorSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		id := network.LayeredID(spec.ConfigLayer, network.OperatorID(spec.Operator, spec.LinkName))

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewOperatorSpec(network.ConfigNamespaceName, id),
			func(r *network.OperatorSpec) error {
				*r.TypedSpec() = spec

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}
