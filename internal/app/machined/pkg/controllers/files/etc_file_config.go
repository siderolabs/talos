// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// EtcFileConfigController renders EtcFileConfig documents into EtcFileSpecs.
type EtcFileConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *EtcFileConfigController) Name() string {
	return "files.EtcFileConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EtcFileConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EtcFileConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EtcFileConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		if err = ctrl.reconcile(ctx, r, cfg); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *EtcFileConfigController) reconcile(ctx context.Context, r controller.Runtime, cfg *config.MachineConfig) error {
	ownedIDs := map[resource.ID]struct{}{}

	if cfg != nil {
		for _, etcFile := range cfg.Config().EtcFileConfigs() {
			if err := safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, etcFile.Name()), func(spec *files.EtcFileSpec) error {
				spec.TypedSpec().Mode = etcFile.Mode()
				spec.TypedSpec().Contents = []byte(etcFile.Content())
				spec.TypedSpec().SelinuxLabel = constants.EtcSelinuxLabel

				return nil
			}); err != nil {
				if state.IsPhaseConflictError(err) {
					ownedIDs[etcFile.Name()] = struct{}{}

					continue
				}

				return fmt.Errorf("failed to write user etc file spec %q: %w", etcFile.Name(), err)
			}

			ownedIDs[etcFile.Name()] = struct{}{}
		}
	}

	specs, err := safe.ReaderList[*files.EtcFileSpec](ctx, r, resource.NewMetadata(files.NamespaceName, files.EtcFileSpecType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing etc file specs: %w", err)
	}

	for spec := range specs.All() {
		if spec.Metadata().Owner() != ctrl.Name() {
			continue
		}

		switch spec.Metadata().Phase() {
		case resource.PhaseRunning:
			if _, ok := ownedIDs[spec.Metadata().ID()]; ok {
				continue
			}

			tornDown, err := r.Teardown(ctx, spec.Metadata())
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error tearing down etc file spec %q: %w", spec.Metadata().ID(), err)
			}

			if !tornDown {
				continue
			}

			if err := r.Destroy(ctx, spec.Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying etc file spec %q: %w", spec.Metadata().ID(), err)
			}
		case resource.PhaseTearingDown:
			if !spec.Metadata().Finalizers().Empty() {
				continue
			}

			if err := r.Destroy(ctx, spec.Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying etc file spec %q: %w", spec.Metadata().ID(), err)
			}
		}
	}

	return nil
}
