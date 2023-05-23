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
	glob "github.com/ryanuber/go-glob"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// DeviceConfigController manages network.DeviceConfig based on configuration.
type DeviceConfigController struct {
	devices map[string]networkDevice
}

//nolint:unused
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

		links, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
		if err != nil {
			return err
		}

		touchedIDs := make(map[resource.ID]struct{})

		var cfgProvider talosconfig.Config

		cfg, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.Config()
		}

		if cfgProvider != nil {
			selectedInterfaces := map[string]struct{}{}

			for index, device := range cfgProvider.Machine().Network().Devices() {
				if device.Selector() != nil {
					dev := device.(*v1alpha1.Device).DeepCopy()
					device = dev

					err = ctrl.getDeviceBySelector(dev, links)
					if err != nil {
						logger.Warn("failed to select an interface for a device", zap.Error(err))

						continue
					}

					if _, ok := selectedInterfaces[device.Interface()]; ok {
						return fmt.Errorf("the device %s is already configured by a selector", device.Interface())
					}

					selectedInterfaces[device.Interface()] = struct{}{}
				}

				if device.Bond() != nil && len(device.Bond().Selectors()) > 0 {
					dev := device.(*v1alpha1.Device).DeepCopy()
					device = dev

					err = ctrl.expandBondSelector(dev, links)
					if err != nil {
						logger.Warn("failed to select interfaces for a bond device", zap.Error(err))

						continue
					}
				}

				id := fmt.Sprintf("%s/%03d", device.Interface(), index)

				touchedIDs[id] = struct{}{}

				config := network.NewDeviceConfig(id, device)

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

		// list network devices for cleanup
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

		r.ResetRestartBackoff()
	}
}

func (ctrl *DeviceConfigController) getDeviceBySelector(device *v1alpha1.Device, links safe.List[*network.LinkStatus]) error {
	selector := device.Selector()

	matches := ctrl.selectDevices(selector, links)
	if len(matches) == 0 {
		return fmt.Errorf("no matching network device for defined selector: %+v", selector)
	}

	link := matches[0]

	device.DeviceInterface = link.Metadata().ID()

	return nil
}

func (ctrl *DeviceConfigController) expandBondSelector(device *v1alpha1.Device, links safe.List[*network.LinkStatus]) error {
	var matches []*network.LinkStatus

	for _, selector := range device.Bond().Selectors() {
		matches = append(matches,
			// filter out bond device itself, as it will inherit the MAC address of the first link
			slices.Filter(
				ctrl.selectDevices(selector, links),
				func(link *network.LinkStatus) bool {
					return link.Metadata().ID() != device.Interface()
				})...)
	}

	device.DeviceBond.BondInterfaces = slices.Map(matches, func(link *network.LinkStatus) string { return link.Metadata().ID() })

	if len(device.DeviceBond.BondInterfaces) == 0 {
		return fmt.Errorf("no matching network device for defined bond selectors: %v",
			slices.Map(device.Bond().Selectors(),
				func(selector talosconfig.NetworkDeviceSelector) string {
					return fmt.Sprintf("%+v", selector)
				},
			),
		)
	}

	device.DeviceBond.BondDeviceSelectors = nil

	return nil
}

func (ctrl *DeviceConfigController) selectDevices(selector talosconfig.NetworkDeviceSelector, links safe.List[*network.LinkStatus]) []*network.LinkStatus {
	var result []*network.LinkStatus

	for iter := safe.IteratorFromList(links); iter.Next(); {
		linkStatus := iter.Value().TypedSpec()

		match := false

		for _, pair := range [][]string{
			{selector.HardwareAddress(), linkStatus.HardwareAddr.String()},
			{selector.PCIID(), linkStatus.PCIID},
			{selector.KernelDriver(), linkStatus.Driver},
			{selector.Bus(), linkStatus.BusPath},
		} {
			if pair[0] == "" {
				continue
			}

			if !glob.Glob(pair[0], pair[1]) {
				match = false

				break
			}

			match = true
		}

		if match {
			result = append(result, iter.Value())
		}
	}

	return result
}
