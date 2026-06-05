// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// LVMLogicalVolumeSpecController translates v1alpha1 LVMLogicalVolumeConfig
// documents into LVMLogicalVolumeSpec desired-state resources.
type LVMLogicalVolumeSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *LVMLogicalVolumeSpecController) Name() string {
	return "storage.LVMLogicalVolumeSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMLogicalVolumeSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMLogicalVolumeSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.LVMLogicalVolumeSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *LVMLogicalVolumeSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

		r.StartTrackingOutputs()

		if machineCfg != nil {
			for _, doc := range machineCfg.Config().LVMLogicalVolumeConfigs() {
				vgName := doc.VolumeGroup()
				lvName := doc.Name()
				id := lvID(vgName + "/" + lvName)

				lvType := doc.Type()
				sizeBytes := doc.MaxSizeBytes()
				sizePercentVG := doc.MaxSizePercentVG()
				mirrors := doc.Mirrors()
				stripes := doc.Stripes()

				if err := safe.WriterModify(
					ctx, r,
					storage.NewLVMLogicalVolumeSpec(storage.NamespaceName, id),
					func(s *storage.LVMLogicalVolumeSpec) error {
						spec := s.TypedSpec()
						spec.VGName = vgName
						spec.Name = lvName
						spec.Type = lvType
						spec.SizeBytes = sizeBytes
						spec.SizePercentVG = sizePercentVG
						spec.Mirrors = mirrors
						spec.Stripes = stripes

						return nil
					},
				); err != nil {
					return fmt.Errorf("modify LVMLogicalVolumeSpec %q: %w", id, err)
				}
			}
		}

		if err := safe.CleanupOutputs[*storage.LVMLogicalVolumeSpec](ctx, r); err != nil {
			return fmt.Errorf("cleanup LVMLogicalVolumeSpec outputs: %w", err)
		}
	}
}
