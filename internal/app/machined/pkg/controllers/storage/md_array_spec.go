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

// MDArraySpecController renders RAIDArrayConfig documents into MDArraySpec.
type MDArraySpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *MDArraySpecController) Name() string {
	return "storage.MDArraySpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MDArraySpecController) Inputs() []controller.Input {
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
func (ctrl *MDArraySpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.MDArraySpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MDArraySpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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
			for _, doc := range machineCfg.Config().RAIDArrayConfigs() {
				name := doc.Name()

				if err := safe.WriterModify(
					ctx, r,
					storage.NewMDArraySpec(storage.NamespaceName, name),
					func(s *storage.MDArraySpec) error {
						spec := s.TypedSpec()
						spec.Name = name
						spec.Level = doc.RAIDLevel()
						spec.VolumeSelector = doc.Provisioning().VolumeSelector()

						return nil
					},
				); err != nil {
					return fmt.Errorf("modify MDArraySpec %q: %w", name, err)
				}
			}
		}

		if err := safe.CleanupOutputs[*storage.MDArraySpec](ctx, r); err != nil {
			return fmt.Errorf("cleanup MDArraySpec outputs: %w", err)
		}
	}
}
