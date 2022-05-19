// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/talos-systems/go-smbios/smbios"
	"go.uber.org/zap"

	hwadapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/hardware"
	"github.com/talos-systems/talos/pkg/machinery/resources/hardware"
)

// SystemInfoController populates CPU information of the underlying hardware.
type SystemInfoController struct {
	SMBIOS *smbios.SMBIOS
}

// Name implements controller.Controller interface.
func (ctrl *SystemInfoController) Name() string {
	return "hardware.SystemInfoController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SystemInfoController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *SystemInfoController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.ProcessorType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: hardware.MemoryType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SystemInfoController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	// controller runs only once
	if ctrl.SMBIOS == nil {
		s, err := GetSMBIOSInfo()
		if err != nil {
			return err
		}

		ctrl.SMBIOS = s
	}

	for _, p := range ctrl.SMBIOS.ProcessorInformation {
		// replaces `CPU 0` with `CPU-0`
		id := strings.ReplaceAll(p.SocketDesignation, " ", "-")

		if err := r.Modify(ctx, hardware.NewProcessorInfo(id), func(res resource.Resource) error {
			hwadapter.Processor(res.(*hardware.Processor)).Update(&p)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	for _, m := range ctrl.SMBIOS.MemoryDevices {
		// replaces `SIMM 0` with `SIMM-0`
		id := strings.ReplaceAll(m.DeviceLocator, " ", "-")

		if err := r.Modify(ctx, hardware.NewMemoryInfo(id), func(res resource.Resource) error {
			hwadapter.Memory(res.(*hardware.Memory)).Update(&m)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	return nil
}
