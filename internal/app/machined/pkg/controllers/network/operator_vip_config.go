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
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/vip"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// OperatorVIPConfigController manages network.OperatorSpec for virtual IPs based on machine configuration.
type OperatorVIPConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *OperatorVIPConfigController) Name() string {
	return "network.OperatorVIPConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *OperatorVIPConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.DeviceConfigSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *OperatorVIPConfigController) Outputs() []controller.Output {
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
func (ctrl *OperatorVIPConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

		devices := xslices.Map(items.Items, func(item resource.Resource) talosconfig.Device {
			return item.(*network.DeviceConfigSpec).TypedSpec().Device
		})

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
		}

		var (
			specs      []network.OperatorSpecSpec
			specErrors *multierror.Error
		)

		// operators from the config
		if len(devices) > 0 {
			for _, device := range devices {
				if device.Ignore() {
					ignoredInterfaces[device.Interface()] = struct{}{}
				}

				if _, ignore := ignoredInterfaces[device.Interface()]; ignore {
					continue
				}

				if device.VIPConfig() != nil {
					if spec, specErr := handleVIP(ctx, device.VIPConfig(), device.Interface(), logger); specErr != nil {
						specErrors = multierror.Append(specErrors, specErr)
					} else {
						specs = append(specs, spec)
					}
				}

				for _, vlan := range device.Vlans() {
					if vlan.VIPConfig() != nil {
						linkName := nethelpers.VLANLinkName(device.Interface(), vlan.ID())
						if spec, specErr := handleVIP(ctx, vlan.VIPConfig(), linkName, logger); specErr != nil {
							specErrors = multierror.Append(specErrors, specErr)
						} else {
							specs = append(specs, spec)
						}
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
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
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
func (ctrl *OperatorVIPConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.OperatorSpecSpec) ([]resource.ID, error) {
	ids := make([]string, 0, len(specs))

	for _, spec := range specs {
		id := network.LayeredID(spec.ConfigLayer, network.OperatorID(spec.Operator, spec.LinkName))

		if err := r.Modify(
			ctx,
			network.NewOperatorSpec(network.ConfigNamespaceName, id),
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

func handleVIP(ctx context.Context, vlanConfig talosconfig.VIPConfig, deviceName string, logger *zap.Logger) (network.OperatorSpecSpec, error) {
	var sharedIP netip.Addr

	sharedIP, err := netip.ParseAddr(vlanConfig.IP())
	if err != nil {
		logger.Warn("ignoring vip parse failure", zap.Error(err), zap.String("link", deviceName))

		return network.OperatorSpecSpec{}, err
	}

	spec := network.OperatorSpecSpec{
		Operator:  network.OperatorVIP,
		LinkName:  deviceName,
		RequireUp: true,
		VIP: network.VIPOperatorSpec{
			IP:            sharedIP,
			GratuitousARP: true,
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	switch {
	// Equinix Metal VIP
	case vlanConfig.EquinixMetal() != nil:
		spec.VIP.GratuitousARP = false
		spec.VIP.EquinixMetal.APIToken = vlanConfig.EquinixMetal().APIToken()

		if err = vip.GetProjectAndDeviceIDs(ctx, &spec.VIP.EquinixMetal); err != nil {
			return network.OperatorSpecSpec{}, err
		}
	// Hetzner Cloud VIP
	case vlanConfig.HCloud() != nil:
		spec.VIP.GratuitousARP = false
		spec.VIP.HCloud.APIToken = vlanConfig.HCloud().APIToken()

		if err = vip.GetNetworkAndDeviceIDs(ctx, &spec.VIP.HCloud, sharedIP); err != nil {
			return network.OperatorSpecSpec{}, err
		}
	// Regular layer 2 VIP
	default:
	}

	return spec, nil
}
