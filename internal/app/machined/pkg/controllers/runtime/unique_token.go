// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// UniqueMachineTokenController is a controller that manages SideroLink unique token.
type UniqueMachineTokenController struct{}

// Name implements controller.Controller interface.
func (ctrl *UniqueMachineTokenController) Name() string {
	return "runtime.UniqueMachineTokenController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UniqueMachineTokenController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			ID:        optional.Some(runtime.MetaKeyTagToID(meta.UniqueMachineToken)),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaLoadedType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UniqueMachineTokenController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.UniqueMachineTokenType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *UniqueMachineTokenController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		metaLoaded, err := safe.ReaderGetByID[*runtime.MetaLoaded](ctx, r, runtime.MetaLoadedID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get meta loaded: %w", err)
		}

		metaKey, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.UniqueMachineToken))
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get unique token meta key: %w", err)
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		if metaLoaded != nil {
			var token string

			if metaKey != nil {
				token = metaKey.TypedSpec().Value
			} else if cfg != nil {
				if cfg.Config().SideroLink() != nil {
					token = cfg.Config().SideroLink().UniqueToken()
				}
			}

			if err = safe.WriterModify(ctx, r, runtime.NewUniqueMachineToken(), func(out *runtime.UniqueMachineToken) error {
				out.TypedSpec().Token = token

				return nil
			}); err != nil {
				return fmt.Errorf("failed to update unique token: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*runtime.UniqueMachineToken](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}
