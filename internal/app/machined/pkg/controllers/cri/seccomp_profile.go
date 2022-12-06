// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// SeccompProfileController manages v1alpha1.Stats which is the current snaphot of the machine CPU and Memory consumption.
type SeccompProfileController struct{}

// Name implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Name() string {
	return "cri.SeccompProfileController"
}

// Inputs implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.StatsController interface.
func (ctrl *SeccompProfileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cri.SeccompProfileType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.StatsController interface.
//
//nolint:gocyclo,cyclop
func (ctrl *SeccompProfileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		touchedIDs := make(map[string]struct{}, len(cfg.Config().Machine().SeccompProfiles()))

		for _, profile := range cfg.Config().Machine().SeccompProfiles() {
			if err = safe.WriterModify(ctx, r, cri.NewSeccompProfile(profile.Name()), func(cri *cri.SeccompProfile) error {
				cri.TypedSpec().Name = profile.Name()
				cri.TypedSpec().Value = profile.Value()

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[profile.Name()] = struct{}{}
		}

		// list keys for cleanup
		list, err := safe.ReaderList[*cri.SeccompProfile](ctx, r, resource.NewMetadata(cri.NamespaceName, cri.SeccompProfileType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing seccomp profiles: %w", err)
		}

		for iter := safe.IteratorFromList(list); iter.Next(); {
			profile := iter.Value()

			if _, ok := touchedIDs[profile.Metadata().ID()]; !ok {
				if err := r.Destroy(ctx, profile.Metadata()); err != nil {
					return fmt.Errorf("error deleting seccomp profile: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
