// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

const (
	targetDeviceSYSFSPath = "/sys/bus/pci/devices/%s"
	driverOverridePath    = targetDeviceSYSFSPath + "/driver_override"
	driverUnbindPath      = targetDeviceSYSFSPath + "/driver/unbind"
	driverPath            = targetDeviceSYSFSPath + "/driver"
	driverProbePath       = "/sys/bus/pci/drivers_probe"
)

// PCIDriverRebindController binds PCI devices to a specific driver and unbinds them from the host driver.
type PCIDriverRebindController struct {
	V1Alpha1Mode v1alpha1runtime.Mode

	boundDevices map[string]struct{}
}

// Name implements controller.Controller interface.
func (c *PCIDriverRebindController) Name() string {
	return "hardware.PCIDriverRebindController"
}

// Inputs implements controller.Controller interface.
func (c *PCIDriverRebindController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (c *PCIDriverRebindController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.PCIDriverRebindStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (c *PCIDriverRebindController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) (err error) {
	// Skip PCI rebind handling if running in a container or agent mode.
	if c.V1Alpha1Mode.InContainer() || c.V1Alpha1Mode.IsAgent() {
		return nil
	}

	if c.boundDevices == nil {
		c.boundDevices = map[string]struct{}{}
	}

	// wait for udevd to be healthy, this is to ensure that host drivers if any are loaded.
	if err := runtimectrl.WaitForDevicesReady(ctx, r,
		[]controller.Input{
			{
				Namespace: hardware.NamespaceName,
				Type:      hardware.PCIDriverRebindConfigType,
				Kind:      controller.InputWeak,
			},
		}); err != nil {
		return fmt.Errorf("error waiting for devices to be ready: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		pciDriverRebindConfigs, err := safe.ReaderListAll[*hardware.PCIDriverRebindConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing all PCI rebind configs: %w", err)
		}

		r.StartTrackingOutputs()

		touchedIDs := map[string]struct{}{}

		for cfg := range pciDriverRebindConfigs.All() {
			if err := c.handlePCIDriverReBind(cfg.TypedSpec().PCIID, cfg.TypedSpec().TargetDriver); err != nil {
				return err
			}

			boundDriver, err := checkDeviceBoundDriver(cfg.TypedSpec().PCIID)
			if err != nil {
				return fmt.Errorf("error checking bound driver for device with id: %s, %w", cfg.TypedSpec().PCIID, err)
			}

			if boundDriver != cfg.TypedSpec().TargetDriver {
				logger.Info(
					"cannot validate if device is bound to target driver, ensure target driver module is loaded",
					zap.String("id", cfg.TypedSpec().PCIID),
					zap.String("targetDriver", cfg.TypedSpec().TargetDriver),
				)
			}

			logger.Info("PCI device bound to target driver", zap.String("id", cfg.TypedSpec().PCIID), zap.String("targetDriver", cfg.TypedSpec().TargetDriver))

			if err := safe.WriterModify[*hardware.PCIDriverRebindStatus](ctx, r, hardware.NewPCIDriverRebindStatus(cfg.TypedSpec().PCIID), func(res *hardware.PCIDriverRebindStatus) error {
				res.TypedSpec().PCIID = cfg.TypedSpec().PCIID
				res.TypedSpec().TargetDriver = cfg.TypedSpec().TargetDriver

				return nil
			}); err != nil {
				return fmt.Errorf("error updating PCI rebind status: %w", err)
			}

			touchedIDs[cfg.TypedSpec().PCIID] = struct{}{}
			c.boundDevices[cfg.TypedSpec().PCIID] = struct{}{}
		}

		// cleanup any PCI devices that were not touched in the current run.
		for pciID := range c.boundDevices {
			if _, ok := touchedIDs[pciID]; !ok {
				// writing a newline to driver_override file will set the device to default driver based on pci device id.
				if err := c.handlePCIDriverReBind(pciID, "\n"); err != nil {
					return err
				}

				logger.Info("PCI device set to default", zap.String("id", pciID))
			}
		}

		if err := safe.CleanupOutputs[*hardware.PCIDriverRebindStatus](ctx, r); err != nil {
			return err
		}
	}
}

// handlePCIBindToTarget binds PCI device to a target driver and unbinds it from the host driver.
func (c *PCIDriverRebindController) handlePCIDriverReBind(pciID, targetDriver string) error {
	if err := os.WriteFile(fmt.Sprintf(driverOverridePath, pciID), []byte(targetDriver), 0o200); err != nil {
		return fmt.Errorf("error writing driver override for device with id: %s, target driver: %s, %w", pciID, targetDriver, err)
	}

	// Unbind device from the host driver.
	// in some cases, the device may not be bound to any driver, so we ignore the error.
	if err := os.WriteFile(fmt.Sprintf(driverUnbindPath, pciID), []byte(pciID), 0o200); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error unbinding device with id: %s, %w", pciID, err)
	}

	if err := os.WriteFile(driverProbePath, []byte(pciID), 0o200); err != nil {
		return fmt.Errorf("error probing driver for device with id: %s, %w", pciID, err)
	}

	return nil
}

// checkDeviceBoundDriver checks if the device is bound to a driver or not bound at all.
func checkDeviceBoundDriver(pciID string) (string, error) {
	driverPath := fmt.Sprintf(driverPath, pciID)

	driver, err := os.Readlink(driverPath)
	if err == nil {
		return filepath.Base(driver), nil
	}

	return "", fmt.Errorf("error reading path: %s, %w", driverPath, err)
}
