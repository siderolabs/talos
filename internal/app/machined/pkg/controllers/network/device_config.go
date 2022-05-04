// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	glob "github.com/ryanuber/go-glob"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// DeviceConfigController manages network.DeviceConfig based on configuration.
type DeviceConfigController struct {
	devices map[string]networkDevice
}

type networkDevice struct {
	hardwareAddress string
	busPrefix       string
	driver          string
	pciID           string
}

// Name implements controller.Controller interface.
func (ctrl *DeviceConfigController) Name() string {
	return "network.DeviceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DeviceConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
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
func (ctrl *DeviceConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.DeviceConfigSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *DeviceConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.devices = map[string]networkDevice{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		links, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
		if err != nil {
			return err
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

		if cfgProvider != nil {
			selectedInterfaces := map[string]struct{}{}

			for _, device := range cfgProvider.Machine().Network().Devices() {
				if device.Selector() != nil {
					err = ctrl.getDeviceBySelector(ctx, device, links.Items)
					if err != nil {
						logger.Warn("failed to select an interface for a device", zap.Error(err))

						continue
					}

					if _, ok := selectedInterfaces[device.Interface()]; ok {
						return fmt.Errorf("the device %s is already configured by a selector", device.Interface())
					}

					selectedInterfaces[device.Interface()] = struct{}{}
				}

				touchedIDs[device.Interface()] = struct{}{}

				config := network.NewDeviceConfig(device)

				if err = r.Modify(
					ctx,
					config,
					func(r resource.Resource) error {
						r.(*network.DeviceConfigSpec).TypedSpec().Device = device

						return nil
					},
				); err != nil {
					return err
				}
			}
		}

		// list links for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.DeviceConfigSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up routes: %w", err)
				}
			}
		}
	}
}

func (ctrl *DeviceConfigController) getDeviceBySelector(ctx context.Context, device talosconfig.Device, links []resource.Resource) error {
	selector := device.Selector()

	for _, link := range links {
		linkStatus := link.(*network.LinkStatus).TypedSpec() //nolint:forcetypeassert,errcheck

		matches := false

		for _, pair := range [][]string{
			{selector.HardwareAddress(), linkStatus.HardwareAddr.String()},
			{selector.PCIID(), linkStatus.PCIID},
			{selector.KernelDriver(), linkStatus.Driver},
			{selector.BusPrefix(), linkStatus.BusPath},
		} {
			if pair[0] == "" {
				continue
			}

			if !glob.Glob(pair[0], pair[1]) {
				matches = false

				break
			}

			matches = true
		}

		if matches {
			dev := device.(*v1alpha1.Device) //nolint:errcheck,forcetypeassert
			dev.DeviceInterface = link.Metadata().ID()

			return nil
		}
	}

	return fmt.Errorf("no matching network device for defined selector: %v", selector)
}
