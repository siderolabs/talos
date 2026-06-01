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
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/vip"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
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
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
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

		devices, err := safe.ReaderListAll[*network.DeviceConfigSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing device config specs: %w", err)
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		linkStatuses, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing link statuses: %w", err)
		}

		linkNameResolver := network.NewLinkResolver(linkStatuses.All)

		ignoredInterfaces := map[string]struct{}{}

		if ctrl.Cmdline != nil {
			var settings CmdlineNetworking

			settings, err = ParseCmdlineNetwork(ctrl.Cmdline, linkNameResolver)
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

		// operators from the legacy config
		for dev := range devices.All() {
			device := dev.TypedSpec().Device

			if device.Ignore() {
				ignoredInterfaces[linkNameResolver.Resolve(device.Interface())] = struct{}{}
			}

			if _, ignore := ignoredInterfaces[linkNameResolver.Resolve(device.Interface())]; ignore {
				continue
			}

			if device.VIPConfig() != nil {
				if spec, specErr := ctrl.handleVIPLegacy(ctx, device.VIPConfig(), linkNameResolver.Resolve(device.Interface()), logger); specErr != nil {
					specErrors = multierror.Append(specErrors, specErr)
				} else {
					specs = append(specs, spec)
				}
			}

			for _, vlan := range device.Vlans() {
				if vlan.VIPConfig() != nil {
					linkName := nethelpers.VLANLinkName(device.Interface(), vlan.ID())
					if spec, specErr := ctrl.handleVIPLegacy(ctx, vlan.VIPConfig(), linkName, logger); specErr != nil {
						specErrors = multierror.Append(specErrors, specErr)
					} else {
						specs = append(specs, spec)
					}
				}
			}
		}

		// new-style config operators
		if cfg != nil {
			for _, doc := range cfg.Config().NetworkVirtualIPConfigs() {
				if spec, specErr := ctrl.handleVIPConfigDoc(ctx, doc, linkNameResolver.Resolve(doc.Link()), logger); specErr != nil {
					specErrors = multierror.Append(specErrors, specErr)
				} else {
					specs = append(specs, spec)
				}
			}
		}

		r.StartTrackingOutputs()

		if err := ctrl.apply(ctx, r, specs); err != nil {
			return fmt.Errorf("error applying operator specs: %w", err)
		}

		// last, check if some specs failed to build; fail last so that other operator specs are applied successfully
		if err = specErrors.ErrorOrNil(); err != nil {
			return err
		}

		if err = r.CleanupOutputs(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.OperatorSpecType, "", resource.VersionUndefined)); err != nil {
			return fmt.Errorf("error cleaning up operator specs: %w", err)
		}
	}
}

//nolint:dupl
func (ctrl *OperatorVIPConfigController) apply(ctx context.Context, r controller.Runtime, specs []network.OperatorSpecSpec) error {
	for _, spec := range specs {
		id := network.LayeredID(spec.ConfigLayer, network.OperatorID(spec))

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewOperatorSpec(network.ConfigNamespaceName, id),
			func(r *network.OperatorSpec) error {
				*r.TypedSpec() = spec

				return nil
			},
		); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *OperatorVIPConfigController) handleVIPLegacy(ctx context.Context, vipConfig talosconfig.VIPConfig, deviceName string, logger *zap.Logger) (network.OperatorSpecSpec, error) {
	var sharedIP netip.Addr

	sharedIP, err := netip.ParseAddr(vipConfig.IP())
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
	case vipConfig.EquinixMetal() != nil:
		spec.VIP.GratuitousARP = false
		spec.VIP.EquinixMetal.APIToken = vipConfig.EquinixMetal().APIToken()

		if err = vip.GetProjectAndDeviceIDs(ctx, &spec.VIP.EquinixMetal); err != nil {
			return network.OperatorSpecSpec{}, err
		}
	// Hetzner Cloud VIP
	case vipConfig.HCloud() != nil:
		spec.VIP.GratuitousARP = false
		spec.VIP.HCloud.APIToken = vipConfig.HCloud().APIToken()

		if err = vip.GetNetworkAndDeviceIDs(ctx, &spec.VIP.HCloud, sharedIP, logger); err != nil {
			return network.OperatorSpecSpec{}, err
		}
	// Regular layer 2 VIP
	default:
	}

	return spec, nil
}

func (ctrl *OperatorVIPConfigController) handleVIPConfigDoc(ctx context.Context, cfg talosconfig.NetworkVirtualIPConfig, deviceName string, logger *zap.Logger) (network.OperatorSpecSpec, error) {
	spec := network.OperatorSpecSpec{
		Operator:  network.OperatorVIP,
		LinkName:  deviceName,
		RequireUp: true,
		VIP: network.VIPOperatorSpec{
			IP:            cfg.VIP(),
			GratuitousARP: true,
		},
		ConfigLayer: network.ConfigMachineConfiguration,
	}

	switch v := cfg.(type) {
	case talosconfig.NetworkHCloudVIPConfig:
		spec.VIP.GratuitousARP = false
		spec.VIP.HCloud.APIToken = v.HCloudAPIToken()

		if err := vip.GetNetworkAndDeviceIDs(ctx, &spec.VIP.HCloud, cfg.VIP(), logger); err != nil {
			return network.OperatorSpecSpec{}, err
		}
	default:
		// nothing to do
	}

	return spec, nil
}
