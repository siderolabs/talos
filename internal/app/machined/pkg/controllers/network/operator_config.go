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
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/network"
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
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
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
func (ctrl *OperatorConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.OperatorSpecType,
			Kind: controller.OutputExclusive,
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
		configuredInterfaces := map[string]struct{}{}

		if ctrl.Cmdline != nil {
			var settings CmdlineNetworking

			settings, err = ParseCmdlineNetwork(ctrl.Cmdline)
			if err != nil {
				logger.Warn("ignored cmdline parse failure", zap.Error(err))
			}

			for _, link := range settings.IgnoreInterfaces {
				ignoredInterfaces[link] = struct{}{}
			}

			if settings.LinkName != "" {
				configuredInterfaces[settings.LinkName] = struct{}{}
			}
		}

		var specs []network.OperatorSpecSpec

		// operators from the config
		if cfgProvider != nil {
			for _, device := range cfgProvider.Machine().Network().Devices() {
				configuredInterfaces[device.Interface()] = struct{}{}

				if device.Ignore() {
					continue
				}

				if _, ignore := ignoredInterfaces[device.Interface()]; ignore {
					continue
				}

				if device.Bond() != nil {
					for _, link := range device.Bond().Interfaces() {
						configuredInterfaces[link] = struct{}{}
					}
				}

				if device.DHCP() && device.DHCPOptions().IPv4() {
					routeMetric := device.DHCPOptions().RouteMetric()
					if routeMetric == 0 {
						routeMetric = DefaultRouteMetric
					}

					specs = append(specs, network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP4,
						LinkName:  device.Interface(),
						RequireUp: true,
						DHCP4: network.DHCP4OperatorSpec{
							RouteMetric: routeMetric,
						},
					})
				}

				if device.DHCP() && device.DHCPOptions().IPv6() {
					routeMetric := device.DHCPOptions().RouteMetric()
					if routeMetric == 0 {
						routeMetric = DefaultRouteMetric
					}

					specs = append(specs, network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP6,
						LinkName:  device.Interface(),
						RequireUp: true,
						DHCP6: network.DHCP6OperatorSpec{
							RouteMetric: routeMetric,
						},
					})
				}

				if device.VIPConfig() != nil {
					var sharedIP netaddr.IP

					sharedIP, err = netaddr.ParseIP(device.VIPConfig().IP())
					if err != nil {
						logger.Warn("ignoring vip parse failure", zap.Error(err), zap.String("link", device.Interface()))
					} else {
						specs = append(specs, network.OperatorSpecSpec{
							Operator:  network.OperatorVIP,
							LinkName:  device.Interface(),
							RequireUp: true,
							VIP: network.VIPOperatorSpec{
								IP: sharedIP,
							},
						})
					}
				}

				if device.WireguardConfig() != nil && device.WireguardConfig().AutomaticNodes() {
					var (
						clusterID string
						prefix    netaddr.IPPrefix
						privKey   string
					)

					clusterID = cfgProvider.Cluster().ID()
					if device.WireguardConfig().ClusterID() != "" {
						clusterID = device.WireguardConfig().ClusterID()
					}

					prefix, err = device.WireguardConfig().AutomaticNodesPrefix()
					if err != nil {
						return fmt.Errorf("failed to retrieve KubeSpan prefix: %w", err)
					}

					privKey, err = device.WireguardConfig().PrivateKey()
					if err != nil {
						return fmt.Errorf("failed to retrieve KubeSpan private key: %w", err)
					}

					specs = append(specs, network.OperatorSpecSpec{
						Operator:  network.OperatorWgLAN,
						LinkName:  device.Interface(),
						RequireUp: true,
						WgLAN: network.WgLANOperatorSpec{
							ClusterID:     clusterID,
							DiscoveryURL:  device.WireguardConfig().NATDiscoveryService(),
							InterfaceName: device.Interface(),
							PodNetworking: device.WireguardConfig().PodNetworkingEnabled(),
							Prefix:        prefix,
							PrivateKey:    privKey,
						},
					})
				}

				for _, vlan := range device.Vlans() {
					if vlan.DHCP() {
						specs = append(specs, network.OperatorSpecSpec{
							Operator:  network.OperatorDHCP4,
							LinkName:  fmt.Sprintf("%s.%d", device.Interface(), vlan.ID()),
							RequireUp: true,
							DHCP4: network.DHCP4OperatorSpec{
								RouteMetric: DefaultRouteMetric,
							},
						})
					}
				}
				// TODO: DHCP6, VIP, WgLAN
			} //nolint:wsl
		}

		// operators from defaults
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing link statuses")
		}

		for _, item := range list.Items {
			linkStatus := item.(*network.LinkStatus) //nolint:errcheck,forcetypeassert

			if linkStatus.Physical() {
				if _, configured := configuredInterfaces[linkStatus.Metadata().ID()]; !configured {
					if _, ignored := ignoredInterfaces[linkStatus.Metadata().ID()]; !ignored {
						// enable DHCPv4 operator on physical interfaces which don't have any explicit configuration and are not ignored
						specs = append(specs, network.OperatorSpecSpec{
							Operator:  network.OperatorDHCP4,
							LinkName:  linkStatus.Metadata().ID(),
							RequireUp: true,
							DHCP4: network.DHCP4OperatorSpec{
								RouteMetric: DefaultRouteMetric,
							},
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
		list, err = r.List(ctx, resource.NewMetadata(network.NamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
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

//nolint:dupl
func (ctrl *OperatorConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.OperatorSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		spec := spec
		id := network.OperatorID(spec.Operator, spec.LinkName)

		if err := r.Modify(
			ctx,
			network.NewOperatorSpec(network.NamespaceName, id),
			func(r resource.Resource) error {
				*r.(*network.OperatorSpec).TypedSpec() = spec

				return nil
			},
		); err != nil {
			return ids, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}
