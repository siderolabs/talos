// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMVolumeGroupSpecController aggregates LVMPhysicalVolumeSpec by VG name,
// emits one LVMVolumeGroupSpec per v1alpha1 doc.
type LVMVolumeGroupSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *LVMVolumeGroupSpecController) Name() string {
	return "storage.LVMVolumeGroupSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMVolumeGroupSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMPhysicalVolumeSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMVolumeGroupSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.LVMVolumeGroupSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LVMVolumeGroupSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineCfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("get machine config: %w", err)
		}

		pvSpecs, err := safe.ReaderListAll[*storage.LVMPhysicalVolumeSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("list LVMPhysicalVolumeSpec: %w", err)
		}

		pvsByVG := map[string][]string{}

		for pv := range pvSpecs.All() {
			spec := pv.TypedSpec()
			pvsByVG[spec.VGName] = append(pvsByVG[spec.VGName], spec.Device)
		}

		r.StartTrackingOutputs()

		if machineCfg != nil {
			for _, doc := range machineCfg.Config().LVMVolumeGroupConfigs() {
				vgName := doc.Name()
				devices := pvsByVG[vgName]
				sort.Strings(devices)

				if err := safe.WriterModify(
					ctx, r,
					storage.NewLVMVolumeGroupSpec(storage.NamespaceName, vgName),
					func(s *storage.LVMVolumeGroupSpec) error {
						spec := s.TypedSpec()
						spec.Name = vgName
						spec.PhysicalVolumes = devices

						return nil
					},
				); err != nil {
					return fmt.Errorf("modify LVMVolumeGroupSpec %q: %w", vgName, err)
				}
			}
		}

		if err := safe.CleanupOutputs[*storage.LVMVolumeGroupSpec](ctx, r); err != nil {
			return fmt.Errorf("cleanup LVMVolumeGroupSpec outputs: %w", err)
		}
	}
}
