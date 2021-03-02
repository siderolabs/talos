// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"log"

	"github.com/AlekSi/pointer"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"

	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// TimeStatusController manages v1alpha1.TimeSync based on configuration and service 'timed' status.
type TimeStatusController struct {
	V1Alpha1State v1alpha1runtime.State
}

// Name implements controller.Controller interface.
func (ctrl *TimeStatusController) Name() string {
	return "v1alpha1.TimeStatusController"
}

// ManagedResources implements controller.Controller interface.
func (ctrl *TimeStatusController) ManagedResources() (resource.Namespace, resource.Type) {
	return v1alpha1.NamespaceName, v1alpha1.TimeStatusType
}

// Run implements controller.Controller interface.
//
//nolint: gocyclo
func (ctrl *TimeStatusController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("timed"),
			Kind:      controller.DependencyWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		var inSync bool

		if cfg.(*config.MachineConfig).Config().Machine().Time().Disabled() {
			// if timed is disabled, time is always "in sync"
			inSync = true
		}

		if ctrl.V1Alpha1State.Platform().Mode() == v1alpha1runtime.ModeContainer {
			// container mode skips timed
			inSync = true
		}

		if !inSync {
			var timedResource resource.Resource

			timedResource, err = r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "timed", resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return err
				}
			} else {
				inSync = timedResource.(*v1alpha1.Service).Healthy()
			}
		}

		if err = r.Update(ctx, v1alpha1.NewTimeStatus(), func(r resource.Resource) error {
			r.(*v1alpha1.TimeStatus).SetSynced(inSync)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}
}
