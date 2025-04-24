// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pcidb/pkg/pcidb"
	"go.uber.org/zap"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// PCIDevicesController populates PCI device information.
type PCIDevicesController struct {
	V1Alpha1Mode runtimetalos.Mode
}

// Name implements controller.Controller interface.
func (ctrl *PCIDevicesController) Name() string {
	return "hardware.PCIDevicesController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PCIDevicesController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PCIDevicesController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.PCIDeviceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *PCIDevicesController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// PCI device info doesn't make sense inside a container, so skip the controller
	if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
		return nil
	}

	// [TODO]: a single run for now, need to figure out how to trigger rescan
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// we need to wait for udevd to be healthy & running so that we get the driver information too
		udevdService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get udevd service: %w", err)
		}

		if udevdService.TypedSpec().Healthy && udevdService.TypedSpec().Running {
			break
		}
	}

	deviceIDs, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return fmt.Errorf("error scanning devices: %w", err)
	}

	logger.Debug("found PCI devices", zap.Int("count", len(deviceIDs)))

	r.StartTrackingOutputs()

	for _, deviceID := range deviceIDs {
		class, err := readHexPCIInfo(deviceID.Name(), "class")
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("error parsing device %s class: %w", deviceID.Name(), err)
		}

		vendor, err := readHexPCIInfo(deviceID.Name(), "vendor")
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("error parsing device %s vendor: %w", deviceID.Name(), err)
		}

		product, err := readHexPCIInfo(deviceID.Name(), "device")
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return fmt.Errorf("error parsing device %s product: %w", deviceID.Name(), err)
		}

		driver, err := readDriverInfo(deviceID.Name())
		if err != nil {
			return fmt.Errorf("error parsing device %s driver: %w", deviceID.Name(), err)
		}

		logger.Debug("found PCI device", zap.String("deviceID", deviceID.Name()), zap.String("driver", driver))

		classID := pcidb.Class((class >> 16) & 0xff)
		subclassID := pcidb.Subclass((class >> 8) & 0xff)
		vendorID := pcidb.Vendor(vendor)
		productID := pcidb.Product(product)

		if err := safe.WriterModify(ctx, r, hardware.NewPCIDeviceInfo(deviceID.Name()), func(r *hardware.PCIDevice) error {
			r.TypedSpec().ClassID = fmt.Sprintf("0x%02x", classID)
			r.TypedSpec().SubclassID = fmt.Sprintf("0x%02x", subclassID)
			r.TypedSpec().VendorID = fmt.Sprintf("0x%04x", vendorID)
			r.TypedSpec().ProductID = fmt.Sprintf("0x%04x", productID)

			r.TypedSpec().Class, _ = pcidb.LookupClass(classID)
			r.TypedSpec().Subclass, _ = pcidb.LookupSubclass(classID, subclassID)
			r.TypedSpec().Vendor, _ = pcidb.LookupVendor(vendorID)
			r.TypedSpec().Product, _ = pcidb.LookupProduct(vendorID, productID)
			r.TypedSpec().Driver = driver

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying output resource: %w", err)
		}
	}

	if err = safe.CleanupOutputs[*hardware.PCIDevice](ctx, r); err != nil {
		return err
	}

	return nil
}

func readHexPCIInfo(deviceID, info string) (uint64, error) {
	contents, err := os.ReadFile(filepath.Join("/sys/bus/pci/devices", deviceID, info))
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(string(bytes.TrimSpace(contents)), 0, 64)
}

func readDriverInfo(deviceID string) (string, error) {
	link, err := os.Readlink(filepath.Join("/sys/bus/pci/devices", deviceID, "driver"))
	if err != nil {
		// ignore if the driver doesn't exist
		// this can happen if the device is not bound to a driver or a pci root port
		if os.IsNotExist(err) {
			return "", nil
		}

		return "", err
	}

	return filepath.Base(link), nil
}
