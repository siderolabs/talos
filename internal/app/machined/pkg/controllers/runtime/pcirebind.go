// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	targetDeviceSYSFSPath = "/sys/bus/pci/devices/%s"
	driverOverridePath    = targetDeviceSYSFSPath + "/driver_override"
	driverUnbindPath      = targetDeviceSYSFSPath + "/driver/unbind"
	driverPath            = targetDeviceSYSFSPath + "/driver"
	driverProbePath       = "/sys/bus/pci/drivers_probe"
)

// PCIRebindController binds PCI devices to a specific driver and unbinds them from the host driver.
type PCIRebindController struct {
	V1Alpha1Mode v1alpha1runtime.Mode

	initialVendorDeviceToBoundDriver map[string]string
	vendorDeviceToBoundDriver        map[string]string
}

// Name implements controller.Controller interface.
func (c *PCIRebindController) Name() string {
	return "runtime.PCIRebindController"
}

// Inputs implements controller.Controller interface.
func (c *PCIRebindController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (c *PCIRebindController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.PCIRebindStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (c *PCIRebindController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) (err error) {
	if c.initialVendorDeviceToBoundDriver == nil {
		c.initialVendorDeviceToBoundDriver = make(map[string]string)
	}

	if c.vendorDeviceToBoundDriver == nil {
		c.vendorDeviceToBoundDriver = make(map[string]string)
	}

	// wait for udevd to be healthy,
	if err := WaitForDevicesReady(ctx, r,
		[]controller.Input{
			{
				Namespace: runtime.NamespaceName,
				Type:      runtime.PCIRebindConfigType,
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

		// Skip PCI rebind handling if running in a container or agent mode.
		if c.V1Alpha1Mode.InContainer() || c.V1Alpha1Mode.IsAgent() {
			return nil
		}

		pciRebindConfigs, err := safe.ReaderListAll[*runtime.PCIRebindConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing all PCI rebind configs: %w", err)
		}

		r.StartTrackingOutputs()

		for cfg := range pciRebindConfigs.All() {
			if _, ok := c.initialVendorDeviceToBoundDriver[cfg.TypedSpec().VendorDeviceID]; !ok {
				boundDriver, err := checkDeviceBoundDriver(cfg.TypedSpec().VendorDeviceID)
				if err != nil {
					return fmt.Errorf("error checking already bound driver for device with id: %s, %w", cfg.TypedSpec().VendorDeviceID, err)
				}

				c.initialVendorDeviceToBoundDriver[cfg.TypedSpec().VendorDeviceID] = boundDriver
			}
		}

		touchedIDs := map[string]struct{}{}

		for cfg := range pciRebindConfigs.All() {
			if targetDriver, ok := c.vendorDeviceToBoundDriver[cfg.TypedSpec().VendorDeviceID]; !ok || targetDriver != cfg.TypedSpec().TargetDriver {
				if err := c.handlePCIBindToTarget(cfg.TypedSpec().VendorDeviceID, cfg.TypedSpec().TargetDriver); err != nil {
					return err
				}

				boundDriver, err := checkDeviceBoundDriver(cfg.TypedSpec().VendorDeviceID)
				if err != nil {
					return fmt.Errorf("error checking bound driver for device with id: %s, %w", cfg.TypedSpec().VendorDeviceID, err)
				}

				if boundDriver != cfg.TypedSpec().TargetDriver { // TODO: this will fail if the module is not loaded
					return fmt.Errorf("error binding device with id: %s to target driver: %s, bound driver: %s", cfg.TypedSpec().VendorDeviceID, cfg.TypedSpec().TargetDriver, boundDriver)
				}

				logger.Info("PCI device bound to target driver", zap.String("vendorDeviceID", cfg.TypedSpec().VendorDeviceID), zap.String("targetDriver", cfg.TypedSpec().TargetDriver))

				if err := safe.WriterModify[*runtime.PCIRebindStatus](ctx, r, runtime.NewPCIRebindStatus(cfg.TypedSpec().Name), func(res *runtime.PCIRebindStatus) error {
					res.TypedSpec().Name = cfg.TypedSpec().Name
					res.TypedSpec().VendorDeviceID = cfg.TypedSpec().VendorDeviceID
					res.TypedSpec().HostDriver = c.initialVendorDeviceToBoundDriver[cfg.TypedSpec().VendorDeviceID]
					res.TypedSpec().TargetDriver = cfg.TypedSpec().TargetDriver

					return nil
				}); err != nil {
					return fmt.Errorf("error updating PCI rebind status: %w", err)
				}

				c.vendorDeviceToBoundDriver[cfg.TypedSpec().VendorDeviceID] = cfg.TypedSpec().TargetDriver
				touchedIDs[cfg.TypedSpec().VendorDeviceID] = struct{}{}
			}
		}

		// cleanup any PCI devices that were not touched in the current run.
		for vendorDeviceID, boundDriver := range c.vendorDeviceToBoundDriver {
			if _, ok := touchedIDs[vendorDeviceID]; !ok {
				if err := c.handlePCIBindToHost(vendorDeviceID, boundDriver); err != nil {
					return err
				}

				logger.Info("PCI device bound to host driver", zap.String("vendorDeviceID", vendorDeviceID), zap.String("hostDriver", boundDriver))

				delete(c.vendorDeviceToBoundDriver, vendorDeviceID)
				delete(c.initialVendorDeviceToBoundDriver, vendorDeviceID)
			}
		}

		if err := safe.CleanupOutputs[*runtime.PCIRebindStatus](ctx, r); err != nil {
			return err
		}
	}
}

// handlePCIBindToTarget binds PCI device to a target driver and unbinds it from the host driver.
func (c *PCIRebindController) handlePCIBindToTarget(vendorDeviceID, targetDriver string) error {
	if err := handleDriverOverride(vendorDeviceID, targetDriver); err != nil {
		return err
	}

	// Unbind device from the host driver.
	// in some cases, the device may not be bound to any driver, so we ignore the error.
	if err := os.WriteFile(fmt.Sprintf(driverUnbindPath, vendorDeviceID), []byte(vendorDeviceID), 0o200); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error unbinding device with id: %s, %w", vendorDeviceID, err)
	}

	return handleDriverProbe(vendorDeviceID)
}

// handlePCIBindToHost unbinds PCI device from a target driver and binds it to the host driver.
func (c *PCIRebindController) handlePCIBindToHost(vendorDeviceID, hostDriver string) error {
	if err := handleDriverOverride(vendorDeviceID, hostDriver); err != nil {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf(driverUnbindPath, vendorDeviceID), []byte(vendorDeviceID), 0o200); err != nil {
		return fmt.Errorf("error unbinding device with id: %s, %w", vendorDeviceID, err)
	}

	return handleDriverProbe(vendorDeviceID)
}

func handleDriverOverride(vendorDeviceID, targetDriver string) error {
	if err := os.WriteFile(fmt.Sprintf(driverOverridePath, vendorDeviceID), []byte(targetDriver), 0o200); err != nil {
		return fmt.Errorf("error writing driver override for device with id: %s, target driver: %s, %w", vendorDeviceID, targetDriver, err)
	}

	return nil
}

func handleDriverProbe(vendorDeviceID string) error {
	if err := os.WriteFile(driverProbePath, []byte(vendorDeviceID), 0o200); err != nil {
		return fmt.Errorf("error probing driver for device with id: %s, %w", vendorDeviceID, err)
	}

	return nil
}

// checkDeviceBoundDriver checks if the device is bound to a driver or not bound at all.
func checkDeviceBoundDriver(vendorDeviceID string) (string, error) {
	driverPath := fmt.Sprintf(driverPath, vendorDeviceID)

	driver, err := os.Readlink(driverPath)
	if err == nil {
		return filepath.Base(driver), nil
	}

	if os.IsNotExist(err) {
		return "", nil
	}

	return "", fmt.Errorf("error reading path: %s, %w", driverPath, err)
}
