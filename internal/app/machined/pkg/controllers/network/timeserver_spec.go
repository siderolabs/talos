// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// TimeServerSpecController applies network.TimeServerSpec to the actual interfaces.
type TimeServerSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *TimeServerSpecController) Name() string {
	return "network.TimeServerSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TimeServerSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.TimeServerSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TimeServerSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.TimeServerStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *TimeServerSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// as there's nothing to do actually apply time servers, simply copy spec to status

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.TimeServerSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// loop over specs and sync to statuses
		for _, res := range list.Items {
			spec := res.(*network.TimeServerSpec) //nolint:forcetypeassert,errcheck

			switch spec.Metadata().Phase() {
			case resource.PhaseTearingDown:
				if err = r.Destroy(ctx, resource.NewMetadata(network.NamespaceName, network.TimeServerStatusType, spec.Metadata().ID(), resource.VersionUndefined)); err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error destroying status: %w", err)
				}

				if err = r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer: %w", err)
				}
			case resource.PhaseRunning:
				ntps := make([]string, len(spec.TypedSpec().NTPServers))

				for i := range ntps {
					ntps[i] = spec.TypedSpec().NTPServers[i]
				}

				logger.Info("setting time servers", zap.Strings("addresses", ntps))

				if err = safe.WriterModify(ctx, r, network.NewTimeServerStatus(network.NamespaceName, spec.Metadata().ID()), func(status *network.TimeServerStatus) error {
					status.TypedSpec().NTPServers = spec.TypedSpec().NTPServers

					return nil
				}); err != nil {
					return fmt.Errorf("error modifying status: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
