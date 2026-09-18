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

	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// StoragePoolSpecController publishes a spec for each configured pool name.
type StoragePoolSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *StoragePoolSpecController) Name() string {
	return "storage.StoragePoolSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StoragePoolSpecController) Inputs() []controller.Input {
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
func (ctrl *StoragePoolSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.StoragePoolSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

func (ctrl *StoragePoolSpecController) writeSpecs(ctx context.Context, r controller.Runtime, cfg configcfg.Config) error {
	for _, doc := range cfg.StoragePoolConfigs() {
		backing, found := configcfg.ResolveBackingVolume(cfg, doc.VolumeName())
		if !found || backing.ReadOnly {
			continue
		}

		if err := safe.WriterModify(
			ctx, r, storage.NewStoragePoolSpec(storage.NamespaceName, doc.Name()),
			func(spec *storage.StoragePoolSpec) error {
				spec.TypedSpec().VolumeID = backing.ID

				return nil
			},
		); err != nil {
			return fmt.Errorf("write pool spec: %w", err)
		}
	}

	return nil
}

// Run implements controller.Controller interface.
func (ctrl *StoragePoolSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("get machine config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg != nil {
			if err = ctrl.writeSpecs(ctx, r, cfg.Config()); err != nil {
				return err
			}
		}

		if err = safe.CleanupOutputs[*storage.StoragePoolSpec](ctx, r); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}
