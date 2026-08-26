// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/ipmi"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// BMCDevicesController discovers baseboard management controllers via the local IPMI interface.
type BMCDevicesController struct {
	V1Alpha1Mode runtimetalos.Mode

	// discovered caches the spec of each device already queried, keyed by device path.
	//
	// Talking to a BMC over KCS is slow (tens of milliseconds per command, and a BMC with no
	// LAN channel configured is only established after probing all of them), while the input
	// fires on every kernel module change - so each device is queried once, and the cached
	// spec is re-applied on subsequent reconciles to keep the resource alive.
	discovered map[string]hardware.BMCDeviceSpec
}

// Name implements controller.Controller interface.
func (ctrl *BMCDevicesController) Name() string {
	return "hardware.BMCDevicesController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BMCDevicesController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelModuleStatusType,
			Kind:      controller.InputWeak,
			ID:        optional.Some("ipmi_si"),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *BMCDevicesController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.BMCDeviceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *BMCDevicesController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// there is no BMC to talk to inside a container
	if ctrl.V1Alpha1Mode.InContainer() {
		return nil
	}

	if ctrl.discovered == nil {
		ctrl.discovered = map[string]hardware.BMCDeviceSpec{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		paths, err := filepath.Glob(ipmi.DevicePathGlob)
		if err != nil {
			return fmt.Errorf("error scanning IPMI devices: %w", err)
		}

		logger.Debug("found IPMI devices", zap.Strings("devices", paths))

		// forget devices which went away, so that they are queried again if they come back
		for path := range ctrl.discovered {
			if !slices.Contains(paths, path) {
				delete(ctrl.discovered, path)
			}
		}

		r.StartTrackingOutputs()

		for _, path := range paths {
			spec, ok := ctrl.discovered[path]
			if !ok {
				if spec, err = ctrl.query(ctx, logger, path); err != nil {
					switch {
					case errors.Is(err, context.Canceled):
						return nil
					case errors.Is(err, unix.EISDIR):
						// the glob also matches the /dev/ipmi directory some udev rulesets create
					default:
						// a BMC which fails to answer shouldn't restart the controller: it would
						// keep hammering the (slow) KCS interface with no chance of succeeding
						logger.Warn("error discovering BMC", zap.String("device", path), zap.Error(err))
					}

					continue
				}

				ctrl.discovered[path] = spec
			}

			if err = safe.WriterModify(ctx, r, hardware.NewBMCDevice(filepath.Base(path)), func(res *hardware.BMCDevice) error {
				*res.TypedSpec() = spec

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying output resource: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*hardware.BMCDevice](ctx, r); err != nil {
			return err
		}
	}
}

// query reads the BMC information from a single IPMI device.
func (ctrl *BMCDevicesController) query(ctx context.Context, logger *zap.Logger, path string) (hardware.BMCDeviceSpec, error) {
	dev, err := ipmi.Open(path)
	if err != nil {
		return hardware.BMCDeviceSpec{}, err
	}

	defer dev.Close() //nolint:errcheck

	deviceInfo, err := ipmi.DeviceID(ctx, dev.SendRecv)
	if err != nil {
		return hardware.BMCDeviceSpec{}, err
	}

	// the network configuration is best-effort: not every BMC has a LAN channel
	// configured, and not every BMC implements the commands to read it
	lanConfig, err := ipmi.FindLANConfig(ctx, dev.SendRecv)
	if err != nil {
		if ctx.Err() != nil {
			return hardware.BMCDeviceSpec{}, err
		}

		logger.Debug("no BMC network configuration", zap.String("device", path), zap.Error(err))
	}

	return hardware.BMCDeviceSpec{
		ManufacturerID:  deviceInfo.ManufacturerID,
		Manufacturer:    ipmi.Vendor(deviceInfo.ManufacturerID),
		ProductID:       uint32(deviceInfo.ProductID),
		FirmwareVersion: deviceInfo.Firmware,
		IPMIVersion:     deviceInfo.IPMIVersion,
		Channel:         uint32(lanConfig.Channel),
		Address:         lanConfig.Address,
		Gateway:         lanConfig.Gateway,
		HardwareAddr:    nethelpers.HardwareAddr(lanConfig.HardwareAddr),
	}, nil
}
