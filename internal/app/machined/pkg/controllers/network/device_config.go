// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	glob "github.com/ryanuber/go-glob"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
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
			ID:        optional.Some(config.V1Alpha1ID),
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

		var cfgProvider talosconfig.Config

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting config: %w", err)
			}
		} else {
			cfgProvider = cfg.Config()
		}

		r.StartTrackingOutputs()

		if cfgProvider != nil && cfgProvider.Machine() != nil {
			for index, device := range cfgProvider.Machine().Network().Devices() {
				out := []talosconfig.Device{device}

				if device.Selector() != nil {
					var matched []*v1alpha1.Device

					matched, err = ctrl.getDevicesBySelector(device, links)
					if err != nil {
						logger.Warn("failed to select an interface for a device", zap.Error(err))

						continue
					}

					out = xslices.Map(matched, func(device *v1alpha1.Device) talosconfig.Device { return device })
				} else if device.Bond() != nil && len(device.Bond().Selectors()) > 0 {
					dev := device.(*v1alpha1.Device).DeepCopy()
					device = dev

					err = ctrl.expandBondSelector(dev, links)
					if err != nil {
						logger.Warn("failed to select interfaces for a bond device", zap.Error(err))

						continue
					}

					out = []talosconfig.Device{device}
				}

				for j, outDevice := range out {
					id := fmt.Sprintf("%s/%03d", outDevice.Interface(), index)

					if len(out) > 1 {
						id = fmt.Sprintf("%s/%03d", id, j)
					}

					if err = safe.WriterModify(
						ctx,
						r,
						network.NewDeviceConfig(id, outDevice),
						func(r *network.DeviceConfigSpec) error {
							r.TypedSpec().Device = outDevice

							return nil
						},
					); err != nil {
						return err
					}
				}
			}
		}

		if err = safe.CleanupOutputs[*network.DeviceConfigSpec](ctx, r); err != nil {
			return err
		}
	}
}

func (ctrl *DeviceConfigController) getDevicesBySelector(device talosconfig.Device, links safe.List[*network.LinkStatus]) ([]*v1alpha1.Device, error) {
	selector := device.Selector()

	matches := ctrl.selectDevices(selector, links)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no matching network device for defined selector: %+v", selector)
	}

	out := make([]*v1alpha1.Device, len(matches))

	for i, link := range matches {
		out[i] = device.(*v1alpha1.Device).DeepCopy()
		out[i].DeviceInterface = link.Metadata().ID()
	}

	return out, nil
}

func (ctrl *DeviceConfigController) expandBondSelector(device *v1alpha1.Device, links safe.List[*network.LinkStatus]) error {
	var matches []*network.LinkStatus

	for _, selector := range device.Bond().Selectors() {
		matches = append(matches,
			// filter out bond device itself, as it will inherit the MAC address of the first link
			xslices.Filter(
				ctrl.selectDevices(selector, links),
				func(link *network.LinkStatus) bool {
					return link.Metadata().ID() != device.Interface()
				})...)
	}

	device.DeviceBond.BondInterfaces = xslices.Map(matches, func(link *network.LinkStatus) string { return link.Metadata().ID() })

	if len(device.DeviceBond.BondInterfaces) == 0 {
		return fmt.Errorf("no matching network device for defined bond selectors: %v",
			xslices.Map(device.Bond().Selectors(),
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

	for linkStatus := range links.All() {
		linkStatusSpec := linkStatus.TypedSpec()

		var match optional.Optional[bool]

		for _, pair := range [][]string{
			{selector.HardwareAddress(), linkStatusSpec.HardwareAddr.String()},
			{selector.PCIID(), linkStatusSpec.PCIID},
			{selector.KernelDriver(), linkStatusSpec.Driver},
			{selector.Bus(), linkStatusSpec.BusPath},
		} {
			if pair[0] == "" {
				continue
			}

			if !glob.Glob(pair[0], pair[1]) {
				match = optional.Some(false)

				break
			}

			match = optional.Some(true)
		}

		if selector.Physical() != nil && match.ValueOr(true) {
			match = optional.Some(*selector.Physical() == linkStatusSpec.Physical())
		}

		if match.ValueOrZero() {
			result = append(result, linkStatus)
		}
	}

	return result
}
