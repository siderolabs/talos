// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

// ServiceController manages v1alpha1.Service based on services subsystem state.
type ServiceController struct {
	V1Alpha1Events runtime.Watcher
}

// Name implements controller.Controller interface.
func (ctrl *ServiceController) Name() string {
	return "v1alpha1.ServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ServiceController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *ServiceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: v1alpha1.ServiceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var wg sync.WaitGroup

	wg.Add(1)

	if err := ctrl.V1Alpha1Events.Watch(func(eventCh <-chan runtime.EventInfo) {
		defer wg.Done()

		for {
			var (
				event runtime.EventInfo
				ok    bool
			)

			select {
			case <-ctx.Done():
				return
			case event, ok = <-eventCh:
				if !ok {
					return
				}
			}

			if msg, ok := event.Payload.(*machine.ServiceStateEvent); ok {
				service := v1alpha1.NewService(msg.Service)

				switch msg.Action { //nolint:exhaustive
				case machine.ServiceStateEvent_RUNNING:
					if err := r.Modify(ctx, service, func(r resource.Resource) error {
						svc := r.(*v1alpha1.Service) //nolint:errcheck,forcetypeassert

						svc.SetRunning(true)
						svc.SetHealthy(msg.GetHealth().GetHealthy())
						svc.SetUnknown(msg.GetHealth().GetUnknown())

						return nil
					}); err != nil {
						logger.Info(fmt.Sprintf("failed creating service resource %s", service), zap.Error(err))
					}
				default:
					if err := r.Destroy(ctx, service.Metadata()); err != nil && !state.IsNotFoundError(err) {
						logger.Info(fmt.Sprintf("failed destroying service resource %s", service), zap.Error(err))
					}
				}
			}
		}
	}, runtime.WithTailEvents(-1)); err != nil {
		return err
	}

	wg.Wait()

	return nil
}
