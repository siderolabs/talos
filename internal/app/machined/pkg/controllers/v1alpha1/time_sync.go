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

// TimeSyncController manages v1alpha1.TimeSync based on configuration and service 'timed' status.
type TimeSyncController struct {
	V1Alpha1State v1alpha1runtime.State
}

// Name implements controller.Controller interface.
func (ctrl *TimeSyncController) Name() string {
	return "v1alpha1.TimeSyncController"
}

// ManagedResources implements controller.Controller interface.
func (ctrl *TimeSyncController) ManagedResources() (resource.Namespace, resource.Type) {
	return v1alpha1.NamespaceName, v1alpha1.TimeSyncType
}

// Run implements controller.Controller interface.
//
//nolint: gocyclo
func (ctrl *TimeSyncController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{
			Namespace: config.NamespaceName,
			Type:      config.V1Alpha1Type,
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

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.V1Alpha1Type, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		var inSync bool

		if cfg.(*config.V1Alpha1).Config().Machine().Time().Disabled() {
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

		if err = r.Update(ctx, v1alpha1.NewTimeSync(), func(r resource.Resource) error {
			r.(*v1alpha1.TimeSync).SetSync(inSync)

			return nil
		}); err != nil {
			return fmt.Errorf("error updating objects: %w", err)
		}
	}
}
