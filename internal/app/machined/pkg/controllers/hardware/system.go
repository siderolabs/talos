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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/prometheus/procfs"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-smbios/smbios"
	"go.uber.org/zap"

	hwadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/hardware"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	pkgSMBIOS "github.com/siderolabs/talos/internal/pkg/smbios"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// SystemInfoController populates CPU information of the underlying hardware.
type SystemInfoController struct {
	V1Alpha1Mode runtimetalos.Mode
	SMBIOS       *smbios.SMBIOS
}

// Name implements controller.Controller interface.
func (ctrl *SystemInfoController) Name() string {
	return "hardware.SystemInfoController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SystemInfoController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaLoadedType,
			ID:        optional.Some(runtime.MetaLoadedID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SystemInfoController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.ProcessorType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: hardware.MemoryModuleType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: hardware.SystemInformationType,
			Kind: controller.OutputExclusive,
		},
	}
}

const memoryModuleUnknown = "UNKNOWN"

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SystemInfoController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// smbios info is not available inside container, so skip the controller
	if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		_, err := safe.ReaderGetByID[*runtime.MetaLoaded](ctx, r, runtime.MetaLoadedID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting meta loaded resource: %w", err)
		}

		if ctrl.SMBIOS == nil {
			var s *smbios.SMBIOS

			s, err = pkgSMBIOS.GetSMBIOSInfo()
			if err != nil {
				return err
			}

			ctrl.SMBIOS = s
		}

		if err := ctrl.reconcileSystemInformation(ctx, r, logger); err != nil {
			return err
		}

		if err := ctrl.reconcileProcessors(ctx, r); err != nil {
			return err
		}

		if err := ctrl.reconcileMemoryModules(ctx, r, logger); err != nil {
			return err
		}

		if err := r.CleanupOutputs(ctx,
			resource.NewMetadata(hardware.NamespaceName, hardware.SystemInformationType, hardware.SystemInformationID, resource.VersionUndefined),
			resource.NewMetadata(hardware.NamespaceName, hardware.ProcessorType, "", resource.VersionUndefined),
			resource.NewMetadata(hardware.NamespaceName, hardware.MemoryModuleType, "", resource.VersionUndefined),
		); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

func (ctrl *SystemInfoController) reconcileSystemInformation(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	uuidRewriteRes, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.UUIDOverride))
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting meta key resource: %w", err)
	}

	var uuidRewrite string

	if uuidRewriteRes != nil && uuidRewriteRes.TypedSpec().Value != "" {
		uuidRewrite = uuidRewriteRes.TypedSpec().Value

		logger.Info("using UUID rewrite", zap.String("uuid", uuidRewrite))
	}

	if err := safe.WriterModify(ctx, r, hardware.NewSystemInformation(hardware.SystemInformationID), func(res *hardware.SystemInformation) error {
		hwadapter.SystemInformation(res).Update(&ctrl.SMBIOS.SystemInformation, uuidRewrite)

		return nil
	}); err != nil {
		return fmt.Errorf("error updating objects: %w", err)
	}

	return nil
}

func (ctrl *SystemInfoController) reconcileProcessors(ctx context.Context, r controller.Runtime) error {
	for _, p := range ctrl.SMBIOS.ProcessorInformation {
		// replaces `CPU 0` with `CPU-0`
		id := strings.ReplaceAll(p.SocketDesignation, " ", "-")

		if err := safe.WriterModify(ctx, r, hardware.NewProcessorInfo(id), func(res *hardware.Processor) error {
			hwadapter.Processor(res).Update(&p)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	return nil
}

func (ctrl *SystemInfoController) reconcileMemoryModules(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for _, m := range ctrl.SMBIOS.MemoryDevices {
		// replaces `SIMM 0` with `SIMM-0`
		id := strings.ReplaceAll(m.DeviceLocator, " ", "-")

		if err := safe.WriterModify(ctx, r, hardware.NewMemoryModuleInfo(id), func(res *hardware.MemoryModule) error {
			hwadapter.MemoryModule(res).Update(&m)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	if len(ctrl.SMBIOS.MemoryDevices) == 0 {
		logger.Debug("no memory devices found, attempting to retrieve memory information from procfs")

		proc, err := procfs.NewDefaultFS()
		if err != nil {
			return err
		}

		info, err := proc.Meminfo()
		if err != nil {
			return err
		}

		if err := safe.WriterModify(ctx, r, hardware.NewMemoryModuleInfo(memoryModuleUnknown), func(res *hardware.MemoryModule) error {
			if info.MemTotalBytes != nil {
				hwadapter.MemoryModule(res).TypedSpec().Size = uint32(*info.MemTotal / 1024)
			}

			hwadapter.MemoryModule(res).TypedSpec().Manufacturer = memoryModuleUnknown

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}

	return nil
}
